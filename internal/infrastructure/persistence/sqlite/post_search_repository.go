package sqlite

import (
	"context"
	"database/sql"
	"math"
	"sort"
	"strings"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.PostSearchRepository = (*PostSearchRepository)(nil)
var _ port.PostSearchIndexer = (*PostSearchRepository)(nil)

const (
	searchTitleWeight   = 3.0
	searchTagWeight     = 2.0
	searchContentWeight = 1.0
	searchPhraseBoost   = 2.5
	searchK1            = 1.2
	searchB             = 0.75
)

type PostSearchRepository struct {
	db               sqlExecutor
	afterRebuildLoad func()
}

func NewPostSearchRepository(db sqlExecutor) *PostSearchRepository {
	return &PostSearchRepository{db: db}
}

func (r *PostSearchRepository) SearchPublishedPosts(ctx context.Context, query string, limit int, cursor *port.PostSearchCursor) ([]port.PostSearchResult, error) {
	if r == nil || r.db == nil {
		return []port.PostSearchResult{}, nil
	}
	if limit <= 0 {
		return []port.PostSearchResult{}, nil
	}
	queryTerms := tokenizeSearchText(query)
	if len(queryTerms) == 0 {
		return []port.PostSearchResult{}, nil
	}
	normalizedPhrase := normalizeSearchText(query)
	matchingIDs, err := r.loadMatchingPostIDs(ctx, queryTerms)
	if err != nil {
		return nil, err
	}
	if len(matchingIDs) == 0 {
		return []port.PostSearchResult{}, nil
	}
	documents, err := r.loadSearchDocuments(ctx, r.db)
	if err != nil {
		return nil, err
	}
	if len(documents) == 0 {
		return []port.PostSearchResult{}, nil
	}
	titleDF := documentFrequency(documents, func(doc searchDocument) []string { return doc.titleTokens })
	tagDF := documentFrequency(documents, func(doc searchDocument) []string { return doc.tagTokens })
	contentDF := documentFrequency(documents, func(doc searchDocument) []string { return doc.contentTokens })
	avgTitleLen := averageFieldLength(documents, func(doc searchDocument) []string { return doc.titleTokens })
	avgTagLen := averageFieldLength(documents, func(doc searchDocument) []string { return doc.tagTokens })
	avgContentLen := averageFieldLength(documents, func(doc searchDocument) []string { return doc.contentTokens })

	results := make([]port.PostSearchResult, 0, len(documents))
	for _, doc := range documents {
		if _, ok := matchingIDs[doc.post.ID]; !ok {
			continue
		}
		if !containsAllTerms(doc.allTerms, queryTerms) {
			continue
		}
		score := weightedBM25(doc.titleTokens, queryTerms, titleDF, avgTitleLen, len(documents), searchTitleWeight)
		score += weightedBM25(doc.tagTokens, queryTerms, tagDF, avgTagLen, len(documents), searchTagWeight)
		score += weightedBM25(doc.contentTokens, queryTerms, contentDF, avgContentLen, len(documents), searchContentWeight)
		score += phraseBoost(doc, normalizedPhrase)
		if cursor != nil && !searchResultAfterCursor(score, doc.post.ID, *cursor) {
			continue
		}
		results = append(results, port.PostSearchResult{
			Post:  clonePost(doc.post),
			Score: score,
		})
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].Post.ID > results[j].Post.ID
		}
		return results[i].Score > results[j].Score
	})
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func (r *PostSearchRepository) RebuildAll(ctx context.Context) error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.withTransaction(ctx, func(exec sqlExecutor) error {
		documents, err := r.loadSearchDocuments(ctx, exec)
		if err != nil {
			return err
		}
		if r.afterRebuildLoad != nil {
			r.afterRebuildLoad()
		}
		if _, err := exec.ExecContext(ctx, `DELETE FROM post_search_fts`); err != nil {
			return err
		}
		return r.replaceSearchDocuments(ctx, exec, documents)
	})
}

func (r *PostSearchRepository) UpsertPost(ctx context.Context, postID int64) error {
	if r == nil || r.db == nil {
		return nil
	}
	document, ok, err := r.loadSearchDocumentByPostID(ctx, postID)
	if err != nil {
		return err
	}
	return r.withTransaction(ctx, func(exec sqlExecutor) error {
		if err := r.deleteSearchDocument(ctx, exec, postID); err != nil {
			return err
		}
		if !ok {
			return nil
		}
		return r.upsertSearchDocument(ctx, exec, document)
	})
}

func (r *PostSearchRepository) DeletePost(ctx context.Context, postID int64) error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.withTransaction(ctx, func(exec sqlExecutor) error {
		return r.deleteSearchDocument(ctx, exec, postID)
	})
}

func (r *PostSearchRepository) withTransaction(ctx context.Context, fn func(exec sqlExecutor) error) error {
	if db, ok := r.db.(*sql.DB); ok {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer func() {
			_ = tx.Rollback()
		}()
		if err := fn(tx); err != nil {
			return err
		}
		return tx.Commit()
	}
	return fn(r.db)
}

func (r *PostSearchRepository) replaceSearchDocuments(ctx context.Context, exec sqlExecutor, documents []searchDocument) error {
	for _, document := range documents {
		if err := r.upsertSearchDocument(ctx, exec, document); err != nil {
			return err
		}
	}
	return nil
}

func (r *PostSearchRepository) upsertSearchDocument(ctx context.Context, exec sqlExecutor, document searchDocument) error {
	if _, err := exec.ExecContext(ctx, `
INSERT INTO post_search_fts (rowid, title, content, tags)
VALUES (?, ?, ?, ?)
`, document.post.ID, document.titleText, document.contentText, document.tagText); err != nil {
		return err
	}
	return nil
}

func (r *PostSearchRepository) deleteSearchDocument(ctx context.Context, exec sqlExecutor, postID int64) error {
	if _, err := exec.ExecContext(ctx, `
DELETE FROM post_search_fts
WHERE rowid = ?
`, postID); err != nil {
		return err
	}
	return nil
}

func (r *PostSearchRepository) loadMatchingPostIDs(ctx context.Context, queryTerms []string) (map[int64]struct{}, error) {
	if len(queryTerms) == 0 {
		return map[int64]struct{}{}, nil
	}
	query := buildFTSQuery(queryTerms)
	if query == "" {
		return map[int64]struct{}{}, nil
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT rowid
FROM post_search_fts
WHERE post_search_fts MATCH ?
ORDER BY rowid ASC
`, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	matchingIDs := make(map[int64]struct{})
	for rows.Next() {
		var postID int64
		if err := rows.Scan(&postID); err != nil {
			return nil, err
		}
		matchingIDs[postID] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return matchingIDs, nil
}

func (r *PostSearchRepository) loadSearchDocumentByPostID(ctx context.Context, postID int64) (searchDocument, bool, error) {
	documents, err := r.querySearchDocuments(ctx, r.db, `
WHERE p.id = ?
AND p.status = 'published'
`, postID)
	if err != nil {
		return searchDocument{}, false, err
	}
	if len(documents) == 0 {
		return searchDocument{}, false, nil
	}
	return documents[0], true, nil
}

type searchDocument struct {
	post          *entity.Post
	titleTokens   []string
	contentTokens []string
	tagTokens     []string
	titleText     string
	contentText   string
	tagText       string
	allTerms      map[string]struct{}
}

func (r *PostSearchRepository) loadSearchDocuments(ctx context.Context, exec sqlExecutor) ([]searchDocument, error) {
	return r.querySearchDocuments(ctx, exec, `WHERE p.status = 'published'`)
}

func (r *PostSearchRepository) querySearchDocuments(ctx context.Context, exec sqlExecutor, filter string, args ...any) ([]searchDocument, error) {
	query := `
SELECT
    p.id,
    p.uuid,
    p.title,
    p.content,
    p.author_id,
    p.board_id,
    p.status,
    p.created_at,
    p.published_at,
    p.updated_at,
    p.deleted_at,
    COALESCE(t.name, '')
FROM posts p
LEFT JOIN post_tags pt ON pt.post_id = p.id AND pt.status = 'active'
LEFT JOIN tags t ON t.id = pt.tag_id
`
	filter = strings.TrimSpace(filter)
	if filter != "" {
		query += filter + "\n"
	}
	query += `ORDER BY p.id ASC, t.id ASC
`
	rows, err := exec.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectSearchDocuments(rows)
}

func collectSearchDocuments(rows *sql.Rows) ([]searchDocument, error) {
	documentsByID := make(map[int64]*searchDocument)
	order := make([]int64, 0)
	for rows.Next() {
		post, tagName, scanErr := scanSearchRow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		document, exists := documentsByID[post.ID]
		if !exists {
			document = &searchDocument{
				post:        clonePost(post),
				tagText:     "",
				allTerms:    make(map[string]struct{}),
				titleText:   normalizeSearchText(post.Title),
				contentText: normalizeSearchText(post.Content),
			}
			document.titleTokens = tokenizeSearchText(document.titleText)
			document.contentTokens = tokenizeSearchText(document.contentText)
			for _, token := range document.titleTokens {
				document.allTerms[token] = struct{}{}
			}
			for _, token := range document.contentTokens {
				document.allTerms[token] = struct{}{}
			}
			documentsByID[post.ID] = document
			order = append(order, post.ID)
		}
		if tagName != "" {
			normalizedTag := normalizeSearchText(tagName)
			if normalizedTag == "" {
				continue
			}
			document.tagText = strings.TrimSpace(document.tagText + " " + normalizedTag)
			tokens := strings.Fields(normalizedTag)
			document.tagTokens = append(document.tagTokens, tokens...)
			for _, token := range tokens {
				document.allTerms[token] = struct{}{}
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	documents := make([]searchDocument, 0, len(order))
	for _, id := range order {
		documents = append(documents, cloneSearchDocument(*documentsByID[id]))
	}
	return documents, nil
}

func buildFTSQuery(terms []string) string {
	parts := make([]string, 0, len(terms))
	for _, term := range terms {
		term = strings.TrimSpace(term)
		if term == "" {
			continue
		}
		escaped := strings.ReplaceAll(term, `"`, `""`)
		parts = append(parts, `"`+escaped+`"`)
	}
	return strings.Join(parts, " AND ")
}

func scanSearchRow(scanner rowScanner) (*entity.Post, string, error) {
	var publishedAt sql.NullInt64
	var deletedAt sql.NullInt64
	var createdAt sql.NullInt64
	var updatedAt sql.NullInt64
	post := &entity.Post{}
	var tagName string
	if err := scanner.Scan(
		&post.ID,
		&post.UUID,
		&post.Title,
		&post.Content,
		&post.AuthorID,
		&post.BoardID,
		&post.Status,
		&createdAt,
		&publishedAt,
		&updatedAt,
		&deletedAt,
		&tagName,
	); err != nil {
		return nil, "", err
	}
	post.CreatedAt = mustParseSQLTimestamp("posts.created_at", createdAt)
	post.UpdatedAt = mustParseSQLTimestamp("posts.updated_at", updatedAt)
	post.PublishedAt = unixNanoToTimePtr(publishedAt)
	post.DeletedAt = unixNanoToTimePtr(deletedAt)
	return post, tagName, nil
}

func tokenizeSearchText(text string) []string {
	normalized := normalizeSearchText(text)
	if normalized == "" {
		return nil
	}
	return strings.Fields(normalized)
}

func normalizeSearchText(text string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(text))), " ")
}

func containsAllTerms(termSet map[string]struct{}, queryTerms []string) bool {
	for _, term := range queryTerms {
		if _, ok := termSet[term]; !ok {
			return false
		}
	}
	return true
}

func documentFrequency(documents []searchDocument, field func(searchDocument) []string) map[string]int {
	df := make(map[string]int)
	for _, doc := range documents {
		seen := make(map[string]struct{})
		for _, token := range field(doc) {
			if _, ok := seen[token]; ok {
				continue
			}
			seen[token] = struct{}{}
			df[token]++
		}
	}
	return df
}

func averageFieldLength(documents []searchDocument, field func(searchDocument) []string) float64 {
	if len(documents) == 0 {
		return 0
	}
	total := 0
	for _, doc := range documents {
		total += len(field(doc))
	}
	return float64(total) / float64(len(documents))
}

func weightedBM25(tokens, queryTerms []string, df map[string]int, avgLen float64, totalDocs int, weight float64) float64 {
	if len(tokens) == 0 || len(queryTerms) == 0 || totalDocs == 0 {
		return 0
	}
	tf := make(map[string]int)
	for _, token := range tokens {
		tf[token]++
	}
	docLen := float64(len(tokens))
	score := 0.0
	for _, term := range queryTerms {
		termTF := tf[term]
		if termTF == 0 {
			continue
		}
		termDF := df[term]
		idf := math.Log(1 + (float64(totalDocs)-float64(termDF)+0.5)/(float64(termDF)+0.5))
		denominator := float64(termTF) + searchK1*(1-searchB+searchB*safeDiv(docLen, avgLen))
		score += idf * ((float64(termTF) * (searchK1 + 1)) / denominator)
	}
	return score * weight
}

func phraseBoost(doc searchDocument, phrase string) float64 {
	if phrase == "" || !strings.Contains(phrase, " ") {
		return 0
	}
	boost := 0.0
	if strings.Contains(doc.titleText, phrase) {
		boost += searchPhraseBoost * searchTitleWeight
	}
	if strings.Contains(doc.tagText, phrase) {
		boost += searchPhraseBoost * searchTagWeight
	}
	if strings.Contains(doc.contentText, phrase) {
		boost += searchPhraseBoost * searchContentWeight
	}
	return boost
}

func safeDiv(value, divisor float64) float64 {
	if divisor == 0 {
		return 0
	}
	return value / divisor
}

func searchResultAfterCursor(score float64, postID int64, cursor port.PostSearchCursor) bool {
	if score < cursor.Score {
		return true
	}
	if score > cursor.Score {
		return false
	}
	return postID < cursor.PostID
}

func cloneSearchDocument(document searchDocument) searchDocument {
	clonedTerms := make(map[string]struct{}, len(document.allTerms))
	for term := range document.allTerms {
		clonedTerms[term] = struct{}{}
	}
	return searchDocument{
		post:          clonePost(document.post),
		titleTokens:   append([]string(nil), document.titleTokens...),
		contentTokens: append([]string(nil), document.contentTokens...),
		tagTokens:     append([]string(nil), document.tagTokens...),
		titleText:     document.titleText,
		contentText:   document.contentText,
		tagText:       document.tagText,
		allTerms:      clonedTerms,
	}
}
