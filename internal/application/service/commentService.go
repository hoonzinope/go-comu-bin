package service

import (
	"strings"

	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
	"github.com/hoonzinope/go-comu-bin/internal/application/cache/key"
	"github.com/hoonzinope/go-comu-bin/internal/application/mapper"
	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.CommentUseCase = (*CommentService)(nil)

type CommentService struct {
	userRepository      port.UserRepository
	postRepository      port.PostRepository
	commentRepository   port.CommentRepository
	reactionRepository  port.ReactionRepository
	cache               port.Cache
	cachePolicy         appcache.Policy
	authorizationPolicy policy.AuthorizationPolicy
}

func NewCommentService(userRepository port.UserRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, cache port.Cache, cachePolicy appcache.Policy, authorizationPolicy policy.AuthorizationPolicy) *CommentService {
	return &CommentService{
		userRepository:      userRepository,
		postRepository:      postRepository,
		commentRepository:   commentRepository,
		reactionRepository:  reactionRepository,
		cache:               cache,
		cachePolicy:         cachePolicy,
		authorizationPolicy: authorizationPolicy,
	}
}

func (s *CommentService) CreateComment(content string, authorID, postID int64, parentID *int64) (int64, error) {
	// 댓글 생성 로직 구현
	if strings.TrimSpace(content) == "" {
		return 0, customError.ErrInvalidInput
	}
	user, err := s.userRepository.SelectUserByID(authorID) // user 존재 여부 확인
	if err != nil {
		return 0, customError.WrapRepository("select user by id for create comment", err)
	}
	if user == nil {
		return 0, customError.ErrUserNotFound
	}
	if err := s.authorizationPolicy.CanWrite(user); err != nil {
		return 0, err
	}
	post, err := s.postRepository.SelectPostByID(postID) // post 존재 여부 확인
	if err != nil {
		return 0, customError.WrapRepository("select post by id for create comment", err)
	}
	if post == nil {
		return 0, customError.ErrPostNotFound
	}
	if parentID != nil {
		parent, err := s.commentRepository.SelectCommentByID(*parentID)
		if err != nil {
			return 0, customError.WrapRepository("select parent comment by id for create comment", err)
		}
		if parent == nil {
			return 0, customError.ErrCommentNotFound
		}
		if parent.PostID != postID {
			return 0, customError.ErrInvalidInput
		}
		if parent.ParentID != nil {
			return 0, customError.ErrInvalidInput
		}
	}
	newComment := entity.NewComment(content, authorID, postID, parentID)
	commentID, err := s.commentRepository.Save(newComment)
	if err != nil {
		return 0, customError.WrapRepository("save comment", err)
	}
	bestEffortCacheDeleteByPrefix(s.cache, key.CommentListPrefix(postID), "invalidate comment list after create comment")
	bestEffortCacheDelete(s.cache, key.PostDetail(postID), "invalidate post detail after create comment")
	return commentID, nil
}

func (s *CommentService) GetCommentsByPost(postID int64, limit int, lastID int64) (*model.CommentList, error) {
	cacheKey := key.CommentList(postID, limit, lastID)
	value, err := s.cache.GetOrSetWithTTL(cacheKey, s.cachePolicy.ListTTLSeconds, func() (interface{}, error) {
		post, err := s.postRepository.SelectPostByID(postID)
		if err != nil {
			return nil, customError.WrapRepository("select post by id for comment list", err)
		}
		if post == nil {
			return nil, customError.ErrPostNotFound
		}

		comments, err := s.visibleCommentsByPost(postID, limit, lastID)
		if err != nil {
			return nil, err
		}
		hasMore := false
		var nextLastID *int64
		if limit >= 0 && len(comments) > limit {
			hasMore = true
			comments = comments[:limit]
		}
		if hasMore && len(comments) > 0 {
			next := comments[len(comments)-1].ID
			nextLastID = &next
		}

		commentModels, err := s.commentsFromEntities(comments)
		if err != nil {
			return nil, err
		}

		return &model.CommentList{
			Comments:   commentModels,
			Limit:      limit,
			LastID:     lastID,
			HasMore:    hasMore,
			NextLastID: nextLastID,
		}, nil
	})
	if err != nil {
		return nil, normalizeCacheLoadError("load comment list cache", err)
	}
	list, ok := value.(*model.CommentList)
	if !ok {
		return nil, customError.Mark(customError.ErrCacheFailure, "decode comment list cache payload")
	}
	return list, nil
}

func (s *CommentService) commentsFromEntities(comments []*entity.Comment) ([]model.Comment, error) {
	out := make([]model.Comment, 0, len(comments))
	for _, comment := range comments {
		authorUUID, err := userUUIDByID(s.userRepository, comment.AuthorID)
		if err != nil {
			return nil, err
		}
		commentModel := mapper.CommentFromEntity(comment)
		commentModel.AuthorUUID = authorUUID
		out = append(out, commentModel)
	}
	return out, nil
}

func (s *CommentService) UpdateComment(id, authorID int64, content string) error {
	// 댓글 수정 로직 구현
	if strings.TrimSpace(content) == "" {
		return customError.ErrInvalidInput
	}
	comment, err := s.commentRepository.SelectCommentByID(id) // comment 존재 여부 확인
	if err != nil {
		return customError.WrapRepository("select comment by id for update comment", err)
	}
	if comment == nil {
		return customError.ErrCommentNotFound
	}
	requester, err := s.userRepository.SelectUserByID(authorID)
	if err != nil {
		return customError.WrapRepository("select user by id for update comment", err)
	}
	if requester == nil {
		return customError.ErrUserNotFound
	}
	if err := s.authorizationPolicy.CanWrite(requester); err != nil {
		return err
	}
	if err := s.authorizationPolicy.OwnerOrAdmin(requester, comment.AuthorID); err != nil {
		return err
	}
	comment.Update(content)
	err = s.commentRepository.Update(comment)
	if err != nil {
		return customError.WrapRepository("update comment", err)
	}
	bestEffortCacheDeleteByPrefix(s.cache, key.CommentListPrefix(comment.PostID), "invalidate comment list after update comment")
	bestEffortCacheDelete(s.cache, key.PostDetail(comment.PostID), "invalidate post detail after update comment")
	return nil
}

func (s *CommentService) DeleteComment(id, authorID int64) error {
	// 댓글 삭제 로직 구현
	comment, err := s.commentRepository.SelectCommentByID(id) // comment 존재 여부 확인
	if err != nil {
		return customError.WrapRepository("select comment by id for delete comment", err)
	}
	if comment == nil {
		return customError.ErrCommentNotFound
	}
	requester, err := s.userRepository.SelectUserByID(authorID)
	if err != nil {
		return customError.WrapRepository("select user by id for delete comment", err)
	}
	if requester == nil {
		return customError.ErrUserNotFound
	}
	if err := s.authorizationPolicy.CanWrite(requester); err != nil {
		return err
	}
	if err := s.authorizationPolicy.OwnerOrAdmin(requester, comment.AuthorID); err != nil {
		return err
	}
	if err := s.commentRepository.Delete(comment.ID); err != nil {
		return customError.WrapRepository("delete comment", err)
	}
	if _, err := s.reactionRepository.DeleteByTarget(comment.ID, entity.ReactionTargetComment); err != nil {
		return customError.WrapRepository("delete comment reactions", err)
	}
	bestEffortCacheDelete(s.cache, key.ReactionList(string(entity.ReactionTargetComment), comment.ID), "invalidate comment reaction list after delete comment")
	bestEffortCacheDeleteByPrefix(s.cache, key.CommentListPrefix(comment.PostID), "invalidate comment list after delete comment")
	bestEffortCacheDelete(s.cache, key.PostDetail(comment.PostID), "invalidate post detail after delete comment")
	return nil
}

func (s *CommentService) visibleCommentsByPost(postID int64, limit int, lastID int64) ([]*entity.Comment, error) {
	comments, err := s.commentRepository.SelectCommentsIncludingDeleted(postID)
	if err != nil {
		return nil, customError.WrapRepository("select comments by post including deleted", err)
	}
	filtered := filterVisibleComments(comments, lastID)
	if limit > 0 && len(filtered) > limit+1 {
		filtered = filtered[:limit+1]
	}
	return filtered, nil
}
