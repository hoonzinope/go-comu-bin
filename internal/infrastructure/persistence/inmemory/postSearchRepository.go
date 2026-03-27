package inmemory

import (
	"context"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.PostSearchRepository = (*PostSearchStore)(nil)
var _ port.PostSearchIndexer = (*PostSearchStore)(nil)

const (
	searchTitleWeight   = 3.0
	searchTagWeight     = 2.0
	searchContentWeight = 1.0
	searchPhraseBoost   = 2.5
	searchK1            = 1.2
	searchB             = 0.75
)

type PostSearchStore struct {
	mu sync.RWMutex

	postRepository    *PostRepository
	boardRepository   *BoardRepository
	tagRepository     *TagRepository
	postTagRepository *PostTagRepository
	documents         map[int64]searchDocument
	index             searchIndexStats
	afterRebuildLoad  func()
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
	lastUpdatedAt time.Time
}

type searchIndexStats struct {
	documentCount     int
	titleTokenCount   int
	tagTokenCount     int
	contentTokenCount int
	titleDF           map[string]int
	tagDF             map[string]int
	contentDF         map[string]int
	termPostings      map[string]map[int64]struct{}
}

func NewPostSearchStore(postRepository *PostRepository, tagRepository *TagRepository, postTagRepository *PostTagRepository) *PostSearchStore {
	return &PostSearchStore{
		postRepository:    postRepository,
		tagRepository:     tagRepository,
		postTagRepository: postTagRepository,
		documents:         make(map[int64]searchDocument),
		index:             newSearchIndexStats(),
	}
}

func NewPostSearchRepository(postRepository *PostRepository, tagRepository *TagRepository, postTagRepository *PostTagRepository) *PostSearchStore {
	return NewPostSearchStore(postRepository, tagRepository, postTagRepository)
}

func (r *PostSearchStore) AttachBoardRepository(boardRepository *BoardRepository) {
	r.boardRepository = boardRepository
}

func (r *PostSearchStore) SearchPublishedPosts(ctx context.Context, query string, limit int, cursor *port.PostSearchCursor) ([]port.PostSearchResult, error) {
	_ = ctx
	if limit <= 0 {
		return []port.PostSearchResult{}, nil
	}
	if r == nil {
		return []port.PostSearchResult{}, nil
	}

	queryTerms := tokenizeSearchText(query)
	if len(queryTerms) == 0 {
		return []port.PostSearchResult{}, nil
	}
	normalizedPhrase := normalizeSearchText(query)
	uniqueQueryTerms := uniqueSearchTerms(queryTerms)

	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.documents) == 0 || r.index.documentCount == 0 {
		return []port.PostSearchResult{}, nil
	}

	candidateIDs := r.index.matchingPostIDs(uniqueQueryTerms)
	if len(candidateIDs) == 0 {
		return []port.PostSearchResult{}, nil
	}

	avgTitleLen := averageFieldLengthFromTotals(r.index.titleTokenCount, r.index.documentCount)
	avgTagLen := averageFieldLengthFromTotals(r.index.tagTokenCount, r.index.documentCount)
	avgContentLen := averageFieldLengthFromTotals(r.index.contentTokenCount, r.index.documentCount)

	results := make([]port.PostSearchResult, 0, len(candidateIDs))
	for _, postID := range candidateIDs {
		doc, ok := r.documents[postID]
		if !ok {
			continue
		}
		score := weightedBM25(doc.titleTokens, queryTerms, r.index.titleDF, avgTitleLen, r.index.documentCount, searchTitleWeight)
		score += weightedBM25(doc.tagTokens, queryTerms, r.index.tagDF, avgTagLen, r.index.documentCount, searchTagWeight)
		score += weightedBM25(doc.contentTokens, queryTerms, r.index.contentDF, avgContentLen, r.index.documentCount, searchContentWeight)
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

func (r *PostSearchStore) RebuildAll(ctx context.Context) error {
	if r == nil {
		return nil
	}
	startedAt := time.Now()
	documents, err := r.loadSearchDocuments(ctx)
	if err != nil {
		return err
	}
	next := make(map[int64]searchDocument, len(documents))
	for _, document := range documents {
		document.lastUpdatedAt = startedAt
		next[document.post.ID] = document
	}
	if r.afterRebuildLoad != nil {
		r.afterRebuildLoad()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, current := range r.documents {
		if current.lastUpdatedAt.After(startedAt) {
			next[id] = current
		}
	}
	r.documents = next
	r.rebuildIndexLocked()
	return nil
}

func (r *PostSearchStore) UpsertPost(ctx context.Context, postID int64) error {
	if r == nil {
		return nil
	}
	document, ok, err := r.loadSearchDocumentByPostID(ctx, postID)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	current, exists := r.documents[postID]
	if exists && current.lastUpdatedAt.After(now) {
		return nil
	}
	if exists {
		r.removeDocumentLocked(current)
		delete(r.documents, postID)
	}
	if !ok {
		return nil
	}
	document.lastUpdatedAt = now
	r.documents[postID] = document
	r.addDocumentLocked(document)
	return nil
}

func (r *PostSearchStore) DeletePost(ctx context.Context, postID int64) error {
	_ = ctx
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	current, exists := r.documents[postID]
	if exists && current.lastUpdatedAt.After(now) {
		return nil
	}
	if exists {
		r.removeDocumentLocked(current)
	}
	delete(r.documents, postID)
	return nil
}

func (r *PostSearchStore) loadSearchDocuments(ctx context.Context) ([]searchDocument, error) {
	if r.postRepository == nil {
		return nil, nil
	}
	repos, err := r.loadSearchRepositories(ctx)
	if err != nil {
		return nil, err
	}

	tagNamesByID := map[int64]string{}
	if r.tagRepository != nil {
		for id, tag := range repos.tagsByID {
			tagNamesByID[id] = tag.Name
		}
	}

	activeTagNamesByPostID := map[int64][]string{}
	if r.postTagRepository != nil {
		for _, relation := range repos.postTags {
			if relation.Status != entity.PostTagStatusActive {
				continue
			}
			tagName, ok := tagNamesByID[relation.TagID]
			if !ok {
				continue
			}
			activeTagNamesByPostID[relation.PostID] = append(activeTagNamesByPostID[relation.PostID], tagName)
		}
	}

	documents := make([]searchDocument, 0, len(repos.postsByID))
	for _, stored := range repos.postsByID {
		document, ok := buildSearchDocument(stored, activeTagNamesByPostID[stored.ID])
		if !ok {
			continue
		}
		documents = append(documents, document)
	}
	return documents, nil
}

func (r *PostSearchStore) loadSearchDocumentByPostID(ctx context.Context, postID int64) (searchDocument, bool, error) {
	repos, err := r.loadSearchRepositories(ctx)
	if err != nil {
		return searchDocument{}, false, err
	}
	post, ok := repos.postsByID[postID]
	if !ok || post == nil {
		return searchDocument{}, false, nil
	}
	tagNames := make([]string, 0)
	for _, relation := range repos.postTags {
		if relation.PostID != postID || relation.Status != entity.PostTagStatusActive {
			continue
		}
		tag, ok := repos.tagsByID[relation.TagID]
		if !ok || tag == nil {
			continue
		}
		tagNames = append(tagNames, tag.Name)
	}
	document, ok := buildSearchDocument(post, tagNames)
	if !ok {
		return searchDocument{}, false, nil
	}
	return document, true, nil
}

func (r *PostSearchStore) rebuildIndexLocked() {
	r.index = newSearchIndexStats()
	for _, document := range r.documents {
		r.index.addDocument(document)
	}
}

func (r *PostSearchStore) addDocumentLocked(document searchDocument) {
	r.index.addDocument(document)
}

func (r *PostSearchStore) removeDocumentLocked(document searchDocument) {
	r.index.removeDocument(document)
}

type searchRepositorySnapshot struct {
	postsByID map[int64]*entity.Post
	tagsByID  map[int64]*entity.Tag
	postTags  []*entity.PostTag
}

func (r *PostSearchStore) loadSearchRepositories(ctx context.Context) (searchRepositorySnapshot, error) {
	_ = ctx
	snapshot := searchRepositorySnapshot{
		postsByID: make(map[int64]*entity.Post),
		tagsByID:  make(map[int64]*entity.Tag),
		postTags:  []*entity.PostTag{},
	}
	if r.postRepository != nil {
		r.postRepository.coordinator.enter()
		defer r.postRepository.coordinator.exit()
		r.postRepository.mu.RLock()
		for id, post := range r.postRepository.postDB.Data {
			snapshot.postsByID[id] = clonePost(post)
		}
		r.postRepository.mu.RUnlock()
	}
	if r.tagRepository != nil {
		r.tagRepository.coordinator.enter()
		defer r.tagRepository.coordinator.exit()
		r.tagRepository.mu.RLock()
		for id, tag := range r.tagRepository.tagDB.Data {
			snapshot.tagsByID[id] = cloneTag(tag)
		}
		r.tagRepository.mu.RUnlock()
	}
	if r.postTagRepository != nil {
		r.postTagRepository.coordinator.enter()
		defer r.postTagRepository.coordinator.exit()
		r.postTagRepository.mu.RLock()
		for _, relation := range r.postTagRepository.data {
			snapshot.postTags = append(snapshot.postTags, clonePostTag(relation))
		}
		r.postTagRepository.mu.RUnlock()
	}
	return snapshot, nil
}

func buildSearchDocument(post *entity.Post, tagNames []string) (searchDocument, bool) {
	if post == nil || post.Status != entity.PostStatusPublished {
		return searchDocument{}, false
	}
	titleText := normalizeSearchText(post.Title)
	contentText := normalizeSearchText(post.Content)
	tagText := normalizeSearchText(strings.Join(tagNames, " "))
	titleTokens := tokenizeSearchText(titleText)
	contentTokens := tokenizeSearchText(contentText)
	tagTokens := tokenizeSearchText(tagText)
	allTerms := make(map[string]struct{}, len(titleTokens)+len(contentTokens)+len(tagTokens))
	for _, token := range titleTokens {
		allTerms[token] = struct{}{}
	}
	for _, token := range contentTokens {
		allTerms[token] = struct{}{}
	}
	for _, token := range tagTokens {
		allTerms[token] = struct{}{}
	}
	return searchDocument{
		post:          clonePost(post),
		titleTokens:   titleTokens,
		contentTokens: contentTokens,
		tagTokens:     tagTokens,
		titleText:     titleText,
		contentText:   contentText,
		tagText:       tagText,
		allTerms:      allTerms,
	}, true
}

func tokenizeSearchText(text string) []string {
	normalized := normalizeSearchText(text)
	if normalized == "" {
		return nil
	}
	return strings.Fields(normalized)
}

func uniqueSearchTerms(tokens []string) []string {
	if len(tokens) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(tokens))
	unique := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		unique = append(unique, token)
	}
	return unique
}

func normalizeSearchText(text string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(text))), " ")
}

func averageFieldLengthFromTotals(totalTokens, documentCount int) float64 {
	if documentCount == 0 {
		return 0
	}
	return float64(totalTokens) / float64(documentCount)
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
		lastUpdatedAt: document.lastUpdatedAt,
	}
}

func newSearchIndexStats() searchIndexStats {
	return searchIndexStats{
		titleDF:      make(map[string]int),
		tagDF:        make(map[string]int),
		contentDF:    make(map[string]int),
		termPostings: make(map[string]map[int64]struct{}),
	}
}

func (s *searchIndexStats) addDocument(document searchDocument) {
	if s == nil {
		return
	}
	s.documentCount++
	s.titleTokenCount += len(document.titleTokens)
	s.tagTokenCount += len(document.tagTokens)
	s.contentTokenCount += len(document.contentTokens)
	s.updateFieldCounts(s.titleDF, document.titleTokens, 1)
	s.updateFieldCounts(s.tagDF, document.tagTokens, 1)
	s.updateFieldCounts(s.contentDF, document.contentTokens, 1)
	for term := range document.allTerms {
		postings := s.termPostings[term]
		if postings == nil {
			postings = make(map[int64]struct{})
			s.termPostings[term] = postings
		}
		postings[document.post.ID] = struct{}{}
	}
}

func (s *searchIndexStats) removeDocument(document searchDocument) {
	if s == nil {
		return
	}
	if s.documentCount > 0 {
		s.documentCount--
	}
	s.titleTokenCount -= len(document.titleTokens)
	s.tagTokenCount -= len(document.tagTokens)
	s.contentTokenCount -= len(document.contentTokens)
	s.updateFieldCounts(s.titleDF, document.titleTokens, -1)
	s.updateFieldCounts(s.tagDF, document.tagTokens, -1)
	s.updateFieldCounts(s.contentDF, document.contentTokens, -1)
	for term := range document.allTerms {
		postings, ok := s.termPostings[term]
		if !ok {
			continue
		}
		delete(postings, document.post.ID)
		if len(postings) == 0 {
			delete(s.termPostings, term)
		}
	}
}

func (s *searchIndexStats) matchingPostIDs(queryTerms []string) []int64 {
	if s == nil || len(queryTerms) == 0 {
		return nil
	}
	baseTerm := ""
	var basePosting map[int64]struct{}
	for _, term := range queryTerms {
		postings, ok := s.termPostings[term]
		if !ok || len(postings) == 0 {
			return nil
		}
		if basePosting == nil || len(postings) < len(basePosting) {
			baseTerm = term
			basePosting = postings
		}
	}
	if basePosting == nil {
		return nil
	}
	candidates := make(map[int64]struct{}, len(basePosting))
	for postID := range basePosting {
		candidates[postID] = struct{}{}
	}
	for _, term := range queryTerms {
		if term == baseTerm {
			continue
		}
		postings := s.termPostings[term]
		for postID := range candidates {
			if _, ok := postings[postID]; !ok {
				delete(candidates, postID)
			}
		}
		if len(candidates) == 0 {
			return nil
		}
	}
	matching := make([]int64, 0, len(candidates))
	for postID := range candidates {
		matching = append(matching, postID)
	}
	return matching
}

func (s *searchIndexStats) updateFieldCounts(field map[string]int, tokens []string, delta int) {
	if len(tokens) == 0 {
		return
	}
	seen := make(map[string]struct{}, len(tokens))
	for _, token := range tokens {
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		field[token] += delta
		if field[token] <= 0 {
			delete(field, token)
		}
	}
}
