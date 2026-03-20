package service

import (
	"context"

	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
	"github.com/hoonzinope/go-comu-bin/internal/application/cache/key"
	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type commentQueryHandler struct {
	userRepository    port.UserRepository
	boardRepository   port.BoardRepository
	postRepository    port.PostRepository
	commentRepository port.CommentRepository
	cache             port.Cache
	cachePolicy       appcache.Policy
}

func newCommentQueryHandler(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, cache port.Cache, cachePolicy appcache.Policy) *commentQueryHandler {
	return &commentQueryHandler{userRepository: userRepository, boardRepository: boardRepository, postRepository: postRepository, commentRepository: commentRepository, cache: cache, cachePolicy: cachePolicy}
}

func (h *commentQueryHandler) GetCommentsByPost(ctx context.Context, postUUID string, limit int, cursor string) (*model.CommentList, error) {
	if err := requirePositiveLimit(limit); err != nil {
		return nil, err
	}
	lastID, err := decodeOpaqueCursor(cursor)
	if err != nil {
		return nil, err
	}
	post, err := h.postRepository.SelectPostByUUID(ctx, postUUID)
	if err != nil {
		return nil, customerror.WrapRepository("select post by uuid for comment list", err)
	}
	if post == nil {
		return nil, customerror.ErrPostNotFound
	}
	cacheKey := key.CommentList(post.ID, limit, lastID)
	value, err := h.cache.GetOrSetWithTTL(ctx, cacheKey, h.cachePolicy.ListTTLSeconds, func(ctx context.Context) (interface{}, error) {
		currentPost, err := h.postRepository.SelectPostByUUID(ctx, postUUID)
		if err != nil {
			return nil, customerror.WrapRepository("select post by uuid for cached comment list", err)
		}
		if currentPost == nil {
			return nil, customerror.ErrPostNotFound
		}
		if err := policy.EnsureBoardVisibleForUser(ctx, h.boardRepository, nil, currentPost.BoardID, customerror.ErrBoardNotFound, "comment board visibility"); err != nil {
			return nil, err
		}
		fetchLimit, err := cursorFetchLimit(limit)
		if err != nil {
			return nil, err
		}
		page, err := loadCursorListPage(ctx, limit, cursor, lastID, func(ctx context.Context) ([]*entity.Comment, error) {
			comments, err := h.commentRepository.SelectVisibleComments(ctx, currentPost.ID, fetchLimit, lastID)
			if err != nil {
				return nil, customerror.WrapRepository("select visible comments by post", err)
			}
			return comments, nil
		}, func(item *entity.Comment) int64 {
			return item.ID
		})
		if err != nil {
			return nil, err
		}
		commentModels, err := h.commentsFromEntities(ctx, currentPost.UUID, page.items)
		if err != nil {
			return nil, err
		}
		return &model.CommentList{Comments: commentModels, Limit: limit, Cursor: page.cursor, HasMore: page.hasMore, NextCursor: page.nextCursor}, nil
	})
	if err != nil {
		return nil, normalizeCacheLoadError("load comment list cache", err)
	}
	list, ok := value.(*model.CommentList)
	if !ok {
		return nil, customerror.Mark(customerror.ErrCacheFailure, "decode comment list cache payload")
	}
	return list, nil
}

func (h *commentQueryHandler) commentsFromEntities(ctx context.Context, postUUID string, comments []*entity.Comment) ([]model.Comment, error) {
	authorUUIDs, err := userUUIDsForComments(ctx, h.userRepository, comments)
	if err != nil {
		return nil, err
	}
	parentUUIDs, err := commentParentUUIDsByID(ctx, h.commentRepository, comments)
	if err != nil {
		return nil, err
	}
	out := make([]model.Comment, 0, len(comments))
	for _, comment := range comments {
		commentModel, err := commentModelFromEntity(comment, postUUID, authorUUIDs, parentUUIDs)
		if err != nil {
			return nil, err
		}
		out = append(out, *commentModel)
	}
	return out, nil
}
