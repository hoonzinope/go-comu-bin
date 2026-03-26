package post

import (
	"context"
	"errors"
	svccommon "github.com/hoonzinope/go-comu-bin/internal/application/service/common"
	"strings"

	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
	"github.com/hoonzinope/go-comu-bin/internal/application/cache/key"
	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type postQueryHandler struct {
	userRepository        port.UserRepository
	boardRepository       port.BoardRepository
	postRepository        port.PostRepository
	postSearchRepository  port.PostSearchRepository
	postRankingRepository port.PostRankingRepository
	tagRepository         port.TagRepository
	postTagRepository     port.PostTagRepository
	attachmentRepository  port.AttachmentRepository
	commentRepository     port.CommentRepository
	reactionRepository    port.ReactionRepository
	cache                 port.Cache
	cachePolicy           appcache.Policy
	postDetailQuery       *postDetailQuery
}

type QueryHandler = postQueryHandler

func newPostQueryHandler(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, postSearchRepository port.PostSearchRepository, postRankingRepository port.PostRankingRepository, tagRepository port.TagRepository, postTagRepository port.PostTagRepository, attachmentRepository port.AttachmentRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, cache port.Cache, cachePolicy appcache.Policy) *postQueryHandler {
	return &postQueryHandler{
		userRepository:        userRepository,
		boardRepository:       boardRepository,
		postRepository:        postRepository,
		postSearchRepository:  postSearchRepository,
		postRankingRepository: postRankingRepository,
		tagRepository:         tagRepository,
		postTagRepository:     postTagRepository,
		attachmentRepository:  attachmentRepository,
		commentRepository:     commentRepository,
		reactionRepository:    reactionRepository,
		cache:                 cache,
		cachePolicy:           cachePolicy,
		postDetailQuery:       newPostDetailQuery(userRepository, boardRepository, postRepository, tagRepository, postTagRepository, attachmentRepository, commentRepository, reactionRepository),
	}
}

func NewQueryHandler(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, postSearchRepository port.PostSearchRepository, postRankingRepository port.PostRankingRepository, tagRepository port.TagRepository, postTagRepository port.PostTagRepository, attachmentRepository port.AttachmentRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, cache port.Cache, cachePolicy appcache.Policy) *QueryHandler {
	return newPostQueryHandler(userRepository, boardRepository, postRepository, postSearchRepository, postRankingRepository, tagRepository, postTagRepository, attachmentRepository, commentRepository, reactionRepository, cache, cachePolicy)
}

func (h *postQueryHandler) GetPostsList(ctx context.Context, boardUUID string, sortValue string, windowValue string, limit int, cursor string) (*model.PostList, error) {
	if err := svccommon.RequirePositiveLimit(limit); err != nil {
		return nil, err
	}
	lastID, err := svccommon.DecodeOpaqueCursor(cursor)
	if err != nil {
		return nil, err
	}
	board, err := h.boardRepository.SelectBoardByUUID(ctx, boardUUID)
	if err != nil {
		return nil, customerror.WrapRepository("select board by uuid for post list", err)
	}
	if board == nil {
		return nil, customerror.ErrBoardNotFound
	}
	if err := policy.EnsureBoardVisible(board, nil); err != nil {
		return nil, err
	}
	sortBy, window, err := normalizeRankingSortWindow(sortValue, windowValue, port.PostFeedSortLatest, true)
	if err != nil {
		return nil, err
	}
	if sortBy == port.PostFeedSortLatest {
		cacheKey := key.PostList(board.ID, limit, lastID)
		value, err := h.cache.GetOrSetWithTTL(ctx, cacheKey, h.cachePolicy.ListTTLSeconds, func(ctx context.Context) (interface{}, error) {
			currentBoard, err := h.boardRepository.SelectBoardByUUID(ctx, boardUUID)
			if err != nil {
				return nil, customerror.WrapRepository("select board by uuid for cached post list", err)
			}
			if currentBoard == nil {
				return nil, customerror.ErrBoardNotFound
			}
			if err := policy.EnsureBoardVisible(currentBoard, nil); err != nil {
				return nil, err
			}
			fetchLimit, err := svccommon.CursorFetchLimit(limit)
			if err != nil {
				return nil, err
			}
			page, err := svccommon.LoadCursorListPage(ctx, limit, cursor, lastID, func(ctx context.Context) ([]*entity.Post, error) {
				posts, err := h.postRepository.SelectPosts(ctx, currentBoard.ID, fetchLimit, lastID)
				if err != nil {
					return nil, customerror.WrapRepository("select posts by board", err)
				}
				return posts, nil
			}, func(item *entity.Post) int64 {
				return item.ID
			})
			if err != nil {
				return nil, err
			}
			postModels, err := h.postsFromEntities(ctx, page.Items)
			if err != nil {
				return nil, err
			}
			return &model.PostList{Posts: postModels, Limit: limit, Cursor: page.Cursor, HasMore: page.HasMore, NextCursor: page.NextCursor}, nil
		})
		if err != nil {
			return nil, svccommon.NormalizeCacheLoadError("load post list cache", err)
		}
		list, ok := value.(*model.PostList)
		if !ok {
			return nil, customerror.Mark(customerror.ErrCacheFailure, "decode post list cache payload")
		}
		return list, nil
	}
	feedCursor, err := decodeFeedCursor(string(sortBy), string(window), cursor)
	if err != nil {
		return nil, err
	}
	cacheKey := key.RankedPostList(board.ID, string(sortBy), string(window), limit, cursor)
	value, err := h.cache.GetOrSetWithTTL(ctx, cacheKey, h.cachePolicy.ListTTLSeconds, func(ctx context.Context) (interface{}, error) {
		return h.loadRankedPosts(ctx, sortBy, window, limit, feedCursor, cursor, func(post *entity.Post) bool {
			return post != nil && post.BoardID == board.ID
		})
	})
	if err != nil {
		return nil, svccommon.NormalizeCacheLoadError("load post list cache", err)
	}
	list, ok := value.(*model.PostList)
	if !ok {
		return nil, customerror.Mark(customerror.ErrCacheFailure, "decode post list cache payload")
	}
	return list, nil
}

func (h *postQueryHandler) GetPostsByTag(ctx context.Context, tagName string, sortValue string, windowValue string, limit int, cursor string) (*model.PostList, error) {
	if err := svccommon.RequirePositiveLimit(limit); err != nil {
		return nil, err
	}
	lastID, err := svccommon.DecodeOpaqueCursor(cursor)
	if err != nil {
		return nil, err
	}
	normalizedName := normalizeTagName(tagName)
	if normalizedName == "" || len(normalizedName) > maxTagLength {
		return nil, customerror.ErrInvalidInput
	}
	tag, err := h.tagRepository.SelectByName(ctx, normalizedName)
	if err != nil {
		return nil, customerror.WrapRepository("select tag by name for post list", err)
	}
	if tag == nil {
		return nil, customerror.ErrTagNotFound
	}
	sortBy, window, err := normalizeRankingSortWindow(sortValue, windowValue, port.PostFeedSortLatest, true)
	if err != nil {
		return nil, err
	}
	if sortBy == port.PostFeedSortLatest {
		cacheKey := key.TagPostList(normalizedName, limit, lastID)
		value, err := h.cache.GetOrSetWithTTL(ctx, cacheKey, h.cachePolicy.ListTTLSeconds, func(ctx context.Context) (interface{}, error) {
			return h.loadPublishedPostsByTag(ctx, normalizedName, limit, lastID, cursor)
		})
		if err != nil {
			return nil, svccommon.NormalizeCacheLoadError("load tag post list cache", err)
		}
		list, ok := value.(*model.PostList)
		if !ok {
			return nil, customerror.Mark(customerror.ErrCacheFailure, "decode tag post list cache payload")
		}
		return list, nil
	}
	feedCursor, err := decodeFeedCursor(string(sortBy), string(window), cursor)
	if err != nil {
		return nil, err
	}
	cacheKey := key.RankedTagPostList(normalizedName, string(sortBy), string(window), limit, cursor)
	value, err := h.cache.GetOrSetWithTTL(ctx, cacheKey, h.cachePolicy.ListTTLSeconds, func(ctx context.Context) (interface{}, error) {
		postIDs, err := h.loadActiveTagPostIDSet(ctx, tag.ID)
		if err != nil {
			return nil, err
		}
		return h.loadRankedPosts(ctx, sortBy, window, limit, feedCursor, cursor, func(post *entity.Post) bool {
			if post == nil {
				return false
			}
			_, ok := postIDs[post.ID]
			return ok
		})
	})
	if err != nil {
		return nil, svccommon.NormalizeCacheLoadError("load tag post list cache", err)
	}
	list, ok := value.(*model.PostList)
	if !ok {
		return nil, customerror.Mark(customerror.ErrCacheFailure, "decode tag post list cache payload")
	}
	return list, nil
}

func (h *postQueryHandler) SearchPosts(ctx context.Context, query string, sortValue string, windowValue string, limit int, cursor string) (*model.PostList, error) {
	if err := svccommon.RequirePositiveLimit(limit); err != nil {
		return nil, err
	}
	normalizedQuery := normalizeSearchQuery(query)
	if normalizedQuery == "" {
		return nil, customerror.ErrInvalidInput
	}
	searchSort, rankingSort, window, err := normalizeSearchSortWindow(sortValue, windowValue)
	if err != nil {
		return nil, err
	}
	searchCursor, err := decodeSearchCursor(searchSort, string(window), cursor)
	if err != nil {
		return nil, err
	}
	cacheKey := key.PostSearchSortedList(normalizedQuery, searchSort, string(window), limit, cursor)
	value, err := h.cache.GetOrSetWithTTL(ctx, cacheKey, h.cachePolicy.ListTTLSeconds, func(ctx context.Context) (interface{}, error) {
		if searchSort == searchSortRelevance {
			return h.loadPublishedPostsBySearch(ctx, normalizedQuery, limit, searchCursor, cursor)
		}
		return h.loadRankedSearchPosts(ctx, normalizedQuery, rankingSort, window, limit, cursor)
	})
	if err != nil {
		return nil, svccommon.NormalizeCacheLoadError("load post search cache", err)
	}
	list, ok := value.(*model.PostList)
	if !ok {
		return nil, customerror.Mark(customerror.ErrCacheFailure, "decode post search cache payload")
	}
	return list, nil
}

func (h *postQueryHandler) GetFeed(ctx context.Context, sortValue string, windowValue string, limit int, cursor string) (*model.PostList, error) {
	if err := svccommon.RequirePositiveLimit(limit); err != nil {
		return nil, err
	}
	sortBy, window, err := normalizeRankingSortWindow(sortValue, windowValue, port.PostFeedSortHot, true)
	if err != nil {
		return nil, err
	}
	feedCursor, err := decodeFeedCursor(string(sortBy), string(window), cursor)
	if err != nil {
		return nil, err
	}
	cacheKey := key.PostFeedList(string(sortBy), string(window), limit, cursor)
	value, err := h.cache.GetOrSetWithTTL(ctx, cacheKey, h.cachePolicy.ListTTLSeconds, func(ctx context.Context) (interface{}, error) {
		return h.loadFeed(ctx, sortBy, window, limit, feedCursor, cursor)
	})
	if err != nil {
		return nil, svccommon.NormalizeCacheLoadError("load post feed cache", err)
	}
	list, ok := value.(*model.PostList)
	if !ok {
		return nil, customerror.Mark(customerror.ErrCacheFailure, "decode post feed cache payload")
	}
	return list, nil
}

func (h *postQueryHandler) GetPostDetail(ctx context.Context, postUUID string) (*model.PostDetail, error) {
	post, err := h.postRepository.SelectPostByUUID(ctx, postUUID)
	if err != nil {
		return nil, customerror.WrapRepository("select post by uuid for post detail cache key", err)
	}
	if post == nil {
		return nil, customerror.ErrPostNotFound
	}
	cacheKey := key.PostDetail(post.ID)
	value, err := h.cache.GetOrSetWithTTL(ctx, cacheKey, h.cachePolicy.DetailTTLSeconds, func(ctx context.Context) (interface{}, error) {
		detail, err := h.postDetailQuery.Load(ctx, post.ID)
		if err != nil {
			return nil, err
		}
		board, err := h.boardRepository.SelectBoardByUUID(ctx, detail.Post.BoardUUID)
		if err != nil {
			return nil, customerror.WrapRepository("select board by uuid for post detail visibility", err)
		}
		if err := policy.EnsureBoardVisible(board, nil); err != nil {
			return nil, customerror.ErrPostNotFound
		}
		return detail, nil
	})
	if err != nil {
		return nil, svccommon.NormalizeCacheLoadError("load post detail cache", err)
	}
	detail, ok := value.(*model.PostDetail)
	if !ok {
		return nil, customerror.Mark(customerror.ErrCacheFailure, "decode post detail cache payload")
	}
	return detail, nil
}

func (h *postQueryHandler) postsFromEntities(ctx context.Context, posts []*entity.Post) ([]model.Post, error) {
	authorUUIDs, err := svccommon.UserUUIDsForPosts(ctx, h.userRepository, posts)
	if err != nil {
		return nil, err
	}
	boardIDs := make([]int64, 0, len(posts))
	seen := make(map[int64]struct{}, len(posts))
	for _, post := range posts {
		if _, ok := seen[post.BoardID]; ok {
			continue
		}
		seen[post.BoardID] = struct{}{}
		boardIDs = append(boardIDs, post.BoardID)
	}
	boardsByID, err := h.boardRepository.SelectBoardsByIDs(ctx, boardIDs)
	if err != nil {
		return nil, customerror.WrapRepository("select boards by ids for posts", err)
	}
	out := make([]model.Post, 0, len(posts))
	for _, post := range posts {
		board := boardsByID[post.BoardID]
		if board == nil {
			return nil, customerror.WrapRepository("select boards by ids including missing", errors.New("post board not found"))
		}
		postModel, err := postModelFromEntity(post, board.UUID, authorUUIDs)
		if err != nil {
			return nil, err
		}
		out = append(out, postModel)
	}
	return out, nil
}

func (h *postQueryHandler) loadPublishedPostsByTag(ctx context.Context, normalizedName string, limit int, lastID int64, cursorValue string) (*model.PostList, error) {
	tag, err := h.tagRepository.SelectByName(ctx, normalizedName)
	if err != nil {
		return nil, customerror.WrapRepository("select tag by name for post list", err)
	}
	if tag == nil {
		return nil, customerror.ErrTagNotFound
	}
	fetchLimit, err := svccommon.CursorFetchLimit(limit)
	if err != nil {
		return nil, err
	}
	cursor := lastID
	visiblePosts := make([]*entity.Post, 0, fetchLimit)
	boardVisibility := make(map[int64]bool)
	for len(visiblePosts) < fetchLimit {
		publishedPosts, err := h.postRepository.SelectPublishedPostsByTagName(ctx, normalizedName, fetchLimit, cursor)
		if err != nil {
			return nil, customerror.WrapRepository("select published posts by tag name", err)
		}
		if len(publishedPosts) == 0 {
			break
		}
		if err := h.resolveBoardVisibility(ctx, publishedPosts, boardVisibility); err != nil {
			return nil, err
		}
		for _, post := range publishedPosts {
			if boardVisibility[post.BoardID] {
				visiblePosts = append(visiblePosts, post)
				if len(visiblePosts) >= fetchLimit {
					break
				}
			}
		}
		if len(visiblePosts) >= fetchLimit || len(publishedPosts) < fetchLimit {
			break
		}
		cursor = publishedPosts[len(publishedPosts)-1].ID
	}
	hasMore := false
	var nextCursor *string
	if len(visiblePosts) > limit {
		hasMore = true
		visiblePosts = visiblePosts[:limit]
	}
	if hasMore && len(visiblePosts) > 0 {
		next := svccommon.EncodeOpaqueCursor(visiblePosts[len(visiblePosts)-1].ID)
		nextCursor = &next
	}
	postModels, err := h.postsFromEntities(ctx, visiblePosts)
	if err != nil {
		return nil, err
	}
	return &model.PostList{Posts: postModels, Limit: limit, Cursor: cursorValue, HasMore: hasMore, NextCursor: nextCursor}, nil
}

func (h *postQueryHandler) resolveBoardVisibility(ctx context.Context, posts []*entity.Post, boardVisibility map[int64]bool) error {
	boardIDsToFetch := make(map[int64]struct{}, len(posts))
	for _, post := range posts {
		if _, cached := boardVisibility[post.BoardID]; cached {
			continue
		}
		boardIDsToFetch[post.BoardID] = struct{}{}
	}
	if len(boardIDsToFetch) == 0 {
		return nil
	}
	uncachedBoardIDs := make([]int64, 0, len(boardIDsToFetch))
	for boardID := range boardIDsToFetch {
		uncachedBoardIDs = append(uncachedBoardIDs, boardID)
	}
	boardsByID, err := h.boardRepository.SelectBoardsByIDs(ctx, uncachedBoardIDs)
	if err != nil {
		return customerror.WrapRepository("select boards by ids for tag post visibility", err)
	}
	for _, boardID := range uncachedBoardIDs {
		boardVisibility[boardID] = policy.EnsureBoardVisible(boardsByID[boardID], nil) == nil
	}
	return nil
}

func (h *postQueryHandler) loadPublishedPostsBySearch(ctx context.Context, normalizedQuery string, limit int, cursor *port.PostSearchCursor, cursorValue string) (*model.PostList, error) {
	if h.postSearchRepository == nil {
		return nil, customerror.WrapRepository("search posts", errors.New("post search repository is not configured"))
	}
	fetchLimit, err := svccommon.CursorFetchLimit(limit)
	if err != nil {
		return nil, err
	}
	currentCursor := cursor
	visibleResults := make([]port.PostSearchResult, 0, fetchLimit)
	boardVisibility := make(map[int64]bool)
	for len(visibleResults) < fetchLimit {
		results, err := h.postSearchRepository.SearchPublishedPosts(ctx, normalizedQuery, fetchLimit, currentCursor)
		if err != nil {
			return nil, customerror.WrapRepository("search published posts", err)
		}
		if len(results) == 0 {
			break
		}
		posts := make([]*entity.Post, 0, len(results))
		for _, result := range results {
			posts = append(posts, result.Post)
		}
		if err := h.resolveBoardVisibility(ctx, posts, boardVisibility); err != nil {
			return nil, err
		}
		for _, result := range results {
			if result.Post == nil {
				continue
			}
			if boardVisibility[result.Post.BoardID] {
				visibleResults = append(visibleResults, result)
				if len(visibleResults) >= fetchLimit {
					break
				}
			}
		}
		if len(visibleResults) >= fetchLimit || len(results) < fetchLimit {
			break
		}
		last := results[len(results)-1]
		currentCursor = &port.PostSearchCursor{Sort: searchSortRelevance, Window: "", Score: last.Score, PostID: last.Post.ID}
	}
	hasMore := false
	var nextCursor *string
	if len(visibleResults) > limit {
		hasMore = true
		visibleResults = visibleResults[:limit]
	}
	if hasMore && len(visibleResults) > 0 {
		last := visibleResults[len(visibleResults)-1]
		next := encodeSearchCursor(searchSortRelevance, "", last.Score, last.Post.ID)
		nextCursor = &next
	}
	posts := make([]*entity.Post, 0, len(visibleResults))
	for _, result := range visibleResults {
		posts = append(posts, result.Post)
	}
	postModels, err := h.postsFromEntities(ctx, posts)
	if err != nil {
		return nil, err
	}
	return &model.PostList{Posts: postModels, Limit: limit, Cursor: cursorValue, HasMore: hasMore, NextCursor: nextCursor}, nil
}

func (h *postQueryHandler) loadFeed(ctx context.Context, sortBy port.PostFeedSort, window port.PostRankingWindow, limit int, cursor *port.PostFeedCursor, cursorValue string) (*model.PostList, error) {
	return h.loadRankedPosts(ctx, sortBy, window, limit, cursor, cursorValue, func(post *entity.Post) bool {
		return post != nil
	})
}

func (h *postQueryHandler) loadRankedPosts(ctx context.Context, sortBy port.PostFeedSort, window port.PostRankingWindow, limit int, cursor *port.PostFeedCursor, cursorValue string, include func(post *entity.Post) bool) (*model.PostList, error) {
	if h.postRankingRepository == nil {
		return nil, customerror.WrapRepository("list post feed", errors.New("post ranking repository is not configured"))
	}
	fetchLimit, err := svccommon.CursorFetchLimit(limit)
	if err != nil {
		return nil, err
	}
	currentCursor := cursor
	visibleResults := make([]port.PostFeedResult, 0, fetchLimit)
	visiblePosts := make([]*entity.Post, 0, fetchLimit)
	boardVisibility := make(map[int64]bool)
	for len(visibleResults) < fetchLimit {
		results, err := h.postRankingRepository.ListFeed(ctx, sortBy, window, fetchLimit, currentCursor)
		if err != nil {
			return nil, customerror.WrapRepository("list post feed", err)
		}
		if len(results) == 0 {
			break
		}
		postIDs := make([]int64, 0, len(results))
		seenPostIDs := make(map[int64]struct{}, len(results))
		for _, result := range results {
			if _, ok := seenPostIDs[result.PostID]; ok {
				continue
			}
			seenPostIDs[result.PostID] = struct{}{}
			postIDs = append(postIDs, result.PostID)
		}
		postsByID, err := h.postRepository.SelectPostsByIDsIncludingUnpublished(ctx, postIDs)
		if err != nil {
			return nil, customerror.WrapRepository("select posts by ids for feed", err)
		}
		posts := make([]*entity.Post, 0, len(postsByID))
		for _, result := range results {
			post := postsByID[result.PostID]
			if post == nil || post.Status != entity.PostStatusPublished {
				continue
			}
			posts = append(posts, post)
		}
		if err := h.resolveBoardVisibility(ctx, posts, boardVisibility); err != nil {
			return nil, err
		}
		for _, result := range results {
			post := postsByID[result.PostID]
			if post != nil && boardVisibility[result.BoardID] && include(post) {
				visibleResults = append(visibleResults, result)
				visiblePosts = append(visiblePosts, post)
				if len(visibleResults) >= fetchLimit {
					break
				}
			}
		}
		if len(visibleResults) >= fetchLimit || len(results) < fetchLimit {
			break
		}
		last := results[len(results)-1]
		currentCursor = &port.PostFeedCursor{
			Sort:                sortBy,
			Window:              window,
			Score:               last.Score,
			PublishedAtUnixNano: last.PublishedAt.UnixNano(),
			PostID:              last.PostID,
		}
	}
	hasMore := false
	var nextCursor *string
	if len(visibleResults) > limit {
		hasMore = true
		visibleResults = visibleResults[:limit]
		visiblePosts = visiblePosts[:limit]
	}
	if hasMore && len(visibleResults) > 0 {
		last := visibleResults[len(visibleResults)-1]
		next := encodeFeedCursor(sortBy, window, last.Score, last.PublishedAt.UnixNano(), last.PostID)
		nextCursor = &next
	}
	postModels, err := h.postsFromEntities(ctx, visiblePosts)
	if err != nil {
		return nil, err
	}
	return &model.PostList{Posts: postModels, Limit: limit, Cursor: cursorValue, HasMore: hasMore, NextCursor: nextCursor}, nil
}

func (h *postQueryHandler) loadRankedSearchPosts(ctx context.Context, normalizedQuery string, sortBy port.PostFeedSort, window port.PostRankingWindow, limit int, cursorValue string) (*model.PostList, error) {
	matchingPostIDs, err := h.loadSearchMatchSet(ctx, normalizedQuery)
	if err != nil {
		return nil, err
	}
	feedCursor, err := decodeFeedCursor(string(sortBy), string(window), cursorValue)
	if err != nil {
		return nil, err
	}
	return h.loadRankedPosts(ctx, sortBy, window, limit, feedCursor, cursorValue, func(post *entity.Post) bool {
		if post == nil {
			return false
		}
		_, ok := matchingPostIDs[post.ID]
		return ok
	})
}

func (h *postQueryHandler) loadSearchMatchSet(ctx context.Context, normalizedQuery string) (map[int64]struct{}, error) {
	if h.postSearchRepository == nil {
		return nil, customerror.WrapRepository("search posts", errors.New("post search repository is not configured"))
	}
	currentCursor := (*port.PostSearchCursor)(nil)
	matches := make(map[int64]struct{})
	for {
		results, err := h.postSearchRepository.SearchPublishedPosts(ctx, normalizedQuery, svccommon.MaxPageLimit, currentCursor)
		if err != nil {
			return nil, customerror.WrapRepository("search published posts", err)
		}
		if len(results) == 0 {
			break
		}
		for _, result := range results {
			if result.Post != nil {
				matches[result.Post.ID] = struct{}{}
			}
		}
		last := results[len(results)-1]
		currentCursor = &port.PostSearchCursor{Sort: searchSortRelevance, Window: "", Score: last.Score, PostID: last.Post.ID}
		if len(results) < svccommon.MaxPageLimit {
			break
		}
	}
	return matches, nil
}

func normalizeRankingSortWindow(sortValue string, windowValue string, defaultSort port.PostFeedSort, allowBest bool) (port.PostFeedSort, port.PostRankingWindow, error) {
	normalized := strings.ToLower(strings.TrimSpace(sortValue))
	if normalized == "" {
		normalized = string(defaultSort)
	}
	switch port.PostFeedSort(normalized) {
	case port.PostFeedSortHot, port.PostFeedSortLatest:
		if strings.TrimSpace(windowValue) != "" {
			return "", "", customerror.ErrInvalidInput
		}
		return port.PostFeedSort(normalized), "", nil
	case port.PostFeedSortBest:
		if !allowBest || strings.TrimSpace(windowValue) != "" {
			return "", "", customerror.ErrInvalidInput
		}
		return port.PostFeedSortBest, "", nil
	case port.PostFeedSortTop:
		window, err := normalizeTopWindow(windowValue)
		if err != nil {
			return "", "", err
		}
		return port.PostFeedSortTop, window, nil
	default:
		return "", "", customerror.ErrInvalidInput
	}
}

const searchSortRelevance = "relevance"

func normalizeSearchSortWindow(sortValue string, windowValue string) (string, port.PostFeedSort, port.PostRankingWindow, error) {
	normalized := strings.ToLower(strings.TrimSpace(sortValue))
	if normalized == "" {
		return searchSortRelevance, "", "", nil
	}
	if normalized == searchSortRelevance {
		if strings.TrimSpace(windowValue) != "" {
			return "", "", "", customerror.ErrInvalidInput
		}
		return searchSortRelevance, "", "", nil
	}
	switch port.PostFeedSort(normalized) {
	case port.PostFeedSortHot, port.PostFeedSortLatest:
		if strings.TrimSpace(windowValue) != "" {
			return "", "", "", customerror.ErrInvalidInput
		}
		return normalized, port.PostFeedSort(normalized), "", nil
	case port.PostFeedSortTop:
		window, err := normalizeTopWindow(windowValue)
		if err != nil {
			return "", "", "", err
		}
		return normalized, port.PostFeedSortTop, window, nil
	default:
		return "", "", "", customerror.ErrInvalidInput
	}
}

func normalizeTopWindow(windowValue string) (port.PostRankingWindow, error) {
	normalized := strings.ToLower(strings.TrimSpace(windowValue))
	if normalized == "" {
		return port.PostRankingWindow7d, nil
	}
	switch port.PostRankingWindow(normalized) {
	case port.PostRankingWindow24h, port.PostRankingWindow7d, port.PostRankingWindow30d, port.PostRankingWindowAll:
		return port.PostRankingWindow(normalized), nil
	default:
		return "", customerror.ErrInvalidInput
	}
}

func (h *postQueryHandler) loadActiveTagPostIDSet(ctx context.Context, tagID int64) (map[int64]struct{}, error) {
	postIDs := make(map[int64]struct{})
	lastID := int64(0)
	for {
		relations, err := h.postTagRepository.SelectActiveByTagID(ctx, tagID, svccommon.MaxPageLimit, lastID)
		if err != nil {
			return nil, customerror.WrapRepository("select active post tags by tag id", err)
		}
		if len(relations) == 0 {
			break
		}
		for _, relation := range relations {
			postIDs[relation.PostID] = struct{}{}
		}
		if len(relations) < svccommon.MaxPageLimit {
			break
		}
		lastID = relations[len(relations)-1].PostID
	}
	return postIDs, nil
}

func normalizeSearchQuery(query string) string {
	parts := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	return strings.Join(parts, " ")
}
