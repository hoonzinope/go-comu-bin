package service

import (
	"errors"
	"strings"

	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
	"github.com/hoonzinope/go-comu-bin/internal/application/cache/key"
	appevent "github.com/hoonzinope/go-comu-bin/internal/application/event"
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
	unitOfWork          port.UnitOfWork
	cache               port.Cache
	actionDispatcher    port.ActionHookDispatcher
	cachePolicy         appcache.Policy
	authorizationPolicy policy.AuthorizationPolicy
	logger              port.Logger
}

func NewCommentService(userRepository port.UserRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, unitOfWork port.UnitOfWork, cache port.Cache, cachePolicy appcache.Policy, authorizationPolicy policy.AuthorizationPolicy, logger ...port.Logger) *CommentService {
	return NewCommentServiceWithActionDispatcher(userRepository, postRepository, commentRepository, reactionRepository, unitOfWork, cache, nil, cachePolicy, authorizationPolicy, logger...)
}

func NewCommentServiceWithActionDispatcher(userRepository port.UserRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, unitOfWork port.UnitOfWork, cache port.Cache, actionDispatcher port.ActionHookDispatcher, cachePolicy appcache.Policy, authorizationPolicy policy.AuthorizationPolicy, logger ...port.Logger) *CommentService {
	return &CommentService{
		userRepository:      userRepository,
		postRepository:      postRepository,
		commentRepository:   commentRepository,
		reactionRepository:  reactionRepository,
		unitOfWork:          unitOfWork,
		cache:               cache,
		actionDispatcher:    resolveActionDispatcher(actionDispatcher),
		cachePolicy:         cachePolicy,
		authorizationPolicy: authorizationPolicy,
		logger:              resolveLogger(logger),
	}
}

// Deprecated: use NewCommentServiceWithActionDispatcher.
func NewCommentServiceWithPublisher(userRepository port.UserRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, unitOfWork port.UnitOfWork, cache port.Cache, publisher port.EventPublisher, cachePolicy appcache.Policy, authorizationPolicy policy.AuthorizationPolicy, logger ...port.Logger) *CommentService {
	return NewCommentServiceWithActionDispatcher(userRepository, postRepository, commentRepository, reactionRepository, unitOfWork, cache, wrapEventPublisherAsActionDispatcher(publisher), cachePolicy, authorizationPolicy, logger...)
}

func (s *CommentService) CreateComment(content string, authorID, postID int64, parentID *int64) (int64, error) {
	// 댓글 생성 로직 구현
	if strings.TrimSpace(content) == "" {
		return 0, customError.ErrInvalidInput
	}
	newComment := entity.NewComment(content, authorID, postID, parentID)
	var commentID int64
	err := s.unitOfWork.WithinTransaction(func(tx port.TxScope) error {
		user, err := tx.UserRepository().SelectUserByID(authorID)
		if err != nil {
			return customError.WrapRepository("select user by id for create comment", err)
		}
		if user == nil {
			return customError.ErrUserNotFound
		}
		if err := s.authorizationPolicy.CanWrite(user); err != nil {
			return err
		}
		post, err := tx.PostRepository().SelectPostByID(postID)
		if err != nil {
			return customError.WrapRepository("select post by id for create comment", err)
		}
		if post == nil {
			return customError.ErrPostNotFound
		}
		if parentID != nil {
			parent, err := tx.CommentRepository().SelectCommentByID(*parentID)
			if err != nil {
				return customError.WrapRepository("select parent comment by id for create comment", err)
			}
			if parent == nil {
				return customError.ErrCommentNotFound
			}
			if parent.PostID != postID || parent.ParentID != nil {
				return customError.ErrInvalidInput
			}
		}
		commentID, err = tx.CommentRepository().Save(newComment)
		if err != nil {
			return customError.WrapRepository("save comment", err)
		}
		if err := dispatchDomainActions(tx, s.actionDispatcher, appevent.NewCommentChanged("created", commentID, postID)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return commentID, nil
}

func (s *CommentService) GetCommentsByPost(postID int64, limit int, lastID int64) (*model.CommentList, error) {
	if err := requirePositiveLimit(limit); err != nil {
		return nil, err
	}
	cacheKey := key.CommentList(postID, limit, lastID)
	value, err := s.cache.GetOrSetWithTTL(cacheKey, s.cachePolicy.ListTTLSeconds, func() (interface{}, error) {
		post, err := s.postRepository.SelectPostByID(postID)
		if err != nil {
			return nil, customError.WrapRepository("select post by id for comment list", err)
		}
		if post == nil {
			return nil, customError.ErrPostNotFound
		}

		comments, err := s.commentRepository.SelectVisibleComments(postID, limit+1, lastID)
		if err != nil {
			return nil, customError.WrapRepository("select visible comments by post", err)
		}
		hasMore := false
		var nextLastID *int64
		if len(comments) > limit {
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
	authorUUIDs, err := userUUIDsForComments(s.userRepository, comments)
	if err != nil {
		return nil, err
	}
	out := make([]model.Comment, 0, len(comments))
	for _, comment := range comments {
		authorUUID, ok := authorUUIDs[comment.AuthorID]
		if !ok {
			return nil, customError.WrapRepository("select users by ids including deleted", errors.New("comment author not found"))
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
	var postID int64
	err := s.unitOfWork.WithinTransaction(func(tx port.TxScope) error {
		comment, err := tx.CommentRepository().SelectCommentByID(id)
		if err != nil {
			return customError.WrapRepository("select comment by id for update comment", err)
		}
		if comment == nil {
			return customError.ErrCommentNotFound
		}
		requester, err := tx.UserRepository().SelectUserByID(authorID)
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
		updatedComment := *comment
		updatedComment.Update(content)
		if err := tx.CommentRepository().Update(&updatedComment); err != nil {
			return customError.WrapRepository("update comment", err)
		}
		postID = updatedComment.PostID
		if err := dispatchDomainActions(tx, s.actionDispatcher, appevent.NewCommentChanged("updated", id, postID)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *CommentService) DeleteComment(id, authorID int64) error {
	// 댓글 삭제 로직 구현
	var commentID, postID int64
	if err := s.unitOfWork.WithinTransaction(func(tx port.TxScope) error {
		comment, err := tx.CommentRepository().SelectCommentByID(id)
		if err != nil {
			return customError.WrapRepository("select comment by id for delete comment", err)
		}
		if comment == nil {
			return customError.ErrCommentNotFound
		}
		requester, err := tx.UserRepository().SelectUserByID(authorID)
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
		if deleteErr := tx.CommentRepository().Delete(comment.ID); deleteErr != nil {
			return customError.WrapRepository("delete comment", deleteErr)
		}
		if _, reactionErr := tx.ReactionRepository().DeleteByTarget(comment.ID, entity.ReactionTargetComment); reactionErr != nil {
			return customError.WrapRepository("delete comment reactions", reactionErr)
		}
		commentID = comment.ID
		postID = comment.PostID
		if err := dispatchDomainActions(tx, s.actionDispatcher, appevent.NewCommentChanged("deleted", commentID, postID)); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}
