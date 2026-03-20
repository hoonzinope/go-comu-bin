package service

import (
	"context"
	"errors"

	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
	"github.com/hoonzinope/go-comu-bin/internal/application/cache/key"
	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type postQueryHandler struct {
	userRepository       port.UserRepository
	boardRepository      port.BoardRepository
	postRepository       port.PostRepository
	tagRepository        port.TagRepository
	postTagRepository    port.PostTagRepository
	attachmentRepository port.AttachmentRepository
	commentRepository    port.CommentRepository
	reactionRepository   port.ReactionRepository
	cache                port.Cache
	cachePolicy          appcache.Policy
	postDetailQuery      *postDetailQuery
}

func newPostQueryHandler(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, tagRepository port.TagRepository, postTagRepository port.PostTagRepository, attachmentRepository port.AttachmentRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, cache port.Cache, cachePolicy appcache.Policy) *postQueryHandler {
	return &postQueryHandler{
		userRepository:       userRepository,
		boardRepository:      boardRepository,
		postRepository:       postRepository,
		tagRepository:        tagRepository,
		postTagRepository:    postTagRepository,
		attachmentRepository: attachmentRepository,
		commentRepository:    commentRepository,
		reactionRepository:   reactionRepository,
		cache:                cache,
		cachePolicy:          cachePolicy,
		postDetailQuery:      newPostDetailQuery(userRepository, boardRepository, postRepository, tagRepository, postTagRepository, attachmentRepository, commentRepository, reactionRepository),
	}
}

func (h *postQueryHandler) GetPostsList(ctx context.Context, boardUUID string, limit int, cursor string) (*model.PostList, error) {
	if err := requirePositiveLimit(limit); err != nil {
		return nil, err
	}
	lastID, err := decodeOpaqueCursor(cursor)
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
		fetchLimit, err := cursorFetchLimit(limit)
		if err != nil {
			return nil, err
		}
		page, err := loadCursorListPage(ctx, limit, cursor, lastID, func(ctx context.Context) ([]*entity.Post, error) {
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
		postModels, err := h.postsFromEntities(ctx, page.items)
		if err != nil {
			return nil, err
		}
		return &model.PostList{Posts: postModels, Limit: limit, Cursor: page.cursor, HasMore: page.hasMore, NextCursor: page.nextCursor}, nil
	})
	if err != nil {
		return nil, normalizeCacheLoadError("load post list cache", err)
	}
	list, ok := value.(*model.PostList)
	if !ok {
		return nil, customerror.Mark(customerror.ErrCacheFailure, "decode post list cache payload")
	}
	return list, nil
}

func (h *postQueryHandler) GetPostsByTag(ctx context.Context, tagName string, limit int, cursor string) (*model.PostList, error) {
	if err := requirePositiveLimit(limit); err != nil {
		return nil, err
	}
	lastID, err := decodeOpaqueCursor(cursor)
	if err != nil {
		return nil, err
	}
	normalizedName := normalizeTagName(tagName)
	if normalizedName == "" || len(normalizedName) > maxTagLength {
		return nil, customerror.ErrInvalidInput
	}
	cacheKey := key.TagPostList(normalizedName, limit, lastID)
	value, err := h.cache.GetOrSetWithTTL(ctx, cacheKey, h.cachePolicy.ListTTLSeconds, func(ctx context.Context) (interface{}, error) {
		return h.loadPublishedPostsByTag(ctx, normalizedName, limit, lastID, cursor)
	})
	if err != nil {
		return nil, normalizeCacheLoadError("load tag post list cache", err)
	}
	list, ok := value.(*model.PostList)
	if !ok {
		return nil, customerror.Mark(customerror.ErrCacheFailure, "decode tag post list cache payload")
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
		return nil, normalizeCacheLoadError("load post detail cache", err)
	}
	detail, ok := value.(*model.PostDetail)
	if !ok {
		return nil, customerror.Mark(customerror.ErrCacheFailure, "decode post detail cache payload")
	}
	return detail, nil
}

func (h *postQueryHandler) postsFromEntities(ctx context.Context, posts []*entity.Post) ([]model.Post, error) {
	authorUUIDs, err := userUUIDsForPosts(ctx, h.userRepository, posts)
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
	fetchLimit, err := cursorFetchLimit(limit)
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
		next := encodeOpaqueCursor(visiblePosts[len(visiblePosts)-1].ID)
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
