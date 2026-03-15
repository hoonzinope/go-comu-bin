package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
	"github.com/hoonzinope/go-comu-bin/internal/application/cache/key"
	appevent "github.com/hoonzinope/go-comu-bin/internal/application/event"
	"github.com/hoonzinope/go-comu-bin/internal/application/mapper"
	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.CommentUseCase = (*CommentService)(nil)

type CommentService struct {
	userRepository      port.UserRepository
	boardRepository     port.BoardRepository
	postRepository      port.PostRepository
	commentRepository   port.CommentRepository
	reactionRepository  port.ReactionRepository
	unitOfWork          port.UnitOfWork
	cache               port.Cache
	actionDispatcher    port.ActionHookDispatcher
	cachePolicy         appcache.Policy
	authorizationPolicy policy.AuthorizationPolicy
	logger              *slog.Logger
}

func NewCommentService(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, unitOfWork port.UnitOfWork, cache port.Cache, cachePolicy appcache.Policy, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *CommentService {
	return NewCommentServiceWithActionDispatcher(userRepository, boardRepository, postRepository, commentRepository, reactionRepository, unitOfWork, cache, nil, cachePolicy, authorizationPolicy, logger...)
}

func NewCommentServiceWithActionDispatcher(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, unitOfWork port.UnitOfWork, cache port.Cache, actionDispatcher port.ActionHookDispatcher, cachePolicy appcache.Policy, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *CommentService {
	return &CommentService{
		userRepository:      userRepository,
		boardRepository:     boardRepository,
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

func (s *CommentService) CreateComment(ctx context.Context, content string, authorID int64, postUUID string, parentUUID *string) (string, error) {
	if strings.TrimSpace(content) == "" {
		return "", customerror.ErrInvalidInput
	}
	post, err := s.postRepository.SelectPostByUUID(ctx, postUUID)
	if err != nil {
		return "", customerror.WrapRepository("select post by uuid for create comment", err)
	}
	if post == nil {
		return "", customerror.ErrPostNotFound
	}
	var parentID *int64
	newComment := entity.NewComment(content, authorID, post.ID, parentID)
	var commentID int64
	var commentUUID string
	err = s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		user, err := tx.UserRepository().SelectUserByID(txCtx, authorID)
		if err != nil {
			return customerror.WrapRepository("select user by id for create comment", err)
		}
		if user == nil {
			return customerror.ErrUserNotFound
		}
		if err := s.authorizationPolicy.CanWrite(user); err != nil {
			return err
		}
		post, err := tx.PostRepository().SelectPostByUUID(txCtx, postUUID)
		if err != nil {
			return customerror.WrapRepository("select post by uuid for create comment", err)
		}
		if post == nil {
			return customerror.ErrPostNotFound
		}
		if err := s.ensureBoardVisibleByPostTx(tx, user, post.ID); err != nil {
			return err
		}
		if parentUUID != nil && strings.TrimSpace(*parentUUID) != "" {
			parent, err := tx.CommentRepository().SelectCommentByUUID(txCtx, strings.TrimSpace(*parentUUID))
			if err != nil {
				return customerror.WrapRepository("select parent comment by uuid for create comment", err)
			}
			if parent == nil {
				return customerror.ErrCommentNotFound
			}
			if parent.PostID != post.ID || parent.ParentID != nil {
				return customerror.ErrInvalidInput
			}
			newComment.ParentID = &parent.ID
		}
		commentID, err = tx.CommentRepository().Save(txCtx, newComment)
		if err != nil {
			return customerror.WrapRepository("save comment", err)
		}
		commentUUID = newComment.UUID
		if err := dispatchDomainActions(tx, s.actionDispatcher, appevent.NewCommentChanged("created", commentID, post.ID)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return commentUUID, nil
}

func (s *CommentService) GetCommentsByPost(ctx context.Context, postUUID string, limit int, cursor string) (*model.CommentList, error) {
	if err := requirePositiveLimit(limit); err != nil {
		return nil, err
	}
	lastID, err := decodeOpaqueCursor(cursor)
	if err != nil {
		return nil, err
	}
	post, err := s.postRepository.SelectPostByUUID(ctx, postUUID)
	if err != nil {
		return nil, customerror.WrapRepository("select post by uuid for comment list", err)
	}
	if post == nil {
		return nil, customerror.ErrPostNotFound
	}
	cacheKey := key.CommentList(post.ID, limit, lastID)
	value, err := s.cache.GetOrSetWithTTL(ctx, cacheKey, s.cachePolicy.ListTTLSeconds, func(ctx context.Context) (interface{}, error) {
		if err := s.ensureBoardVisible(ctx, nil, post.BoardID); err != nil {
			return nil, err
		}

		comments, err := s.commentRepository.SelectVisibleComments(ctx, post.ID, limit+1, lastID)
		if err != nil {
			return nil, customerror.WrapRepository("select visible comments by post", err)
		}
		hasMore := false
		var nextCursor *string
		if len(comments) > limit {
			hasMore = true
			comments = comments[:limit]
		}
		if hasMore && len(comments) > 0 {
			next := encodeOpaqueCursor(comments[len(comments)-1].ID)
			nextCursor = &next
		}

		commentModels, err := s.commentsFromEntities(ctx, post.UUID, comments)
		if err != nil {
			return nil, err
		}

		return &model.CommentList{
			Comments:   commentModels,
			Limit:      limit,
			Cursor:     cursor,
			HasMore:    hasMore,
			NextCursor: nextCursor,
		}, nil
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

func (s *CommentService) commentsFromEntities(ctx context.Context, postUUID string, comments []*entity.Comment) ([]model.Comment, error) {
	authorUUIDs, err := userUUIDsForComments(ctx, s.userRepository, comments)
	if err != nil {
		return nil, err
	}
	parentUUIDs := map[int64]string{}
	if len(comments) > 0 {
		allComments, err := s.commentRepository.SelectCommentsIncludingDeleted(ctx, comments[0].PostID)
		if err != nil {
			return nil, customerror.WrapRepository("select comments including deleted for parent uuid mapping", err)
		}
		for _, item := range allComments {
			parentUUIDs[item.ID] = item.UUID
		}
	}
	out := make([]model.Comment, 0, len(comments))
	for _, comment := range comments {
		authorUUID, ok := authorUUIDs[comment.AuthorID]
		if !ok {
			return nil, customerror.WrapRepository("select users by ids including deleted", errors.New("comment author not found"))
		}
		commentModel := mapper.CommentFromEntity(comment)
		commentModel.AuthorUUID = authorUUID
		commentModel.PostUUID = postUUID
		if comment.ParentID != nil {
			if parentUUID, ok := parentUUIDs[*comment.ParentID]; ok {
				commentModel.ParentUUID = &parentUUID
			}
		}
		out = append(out, commentModel)
	}
	return out, nil
}

func (s *CommentService) UpdateComment(ctx context.Context, commentUUID string, authorID int64, content string) error {
	if strings.TrimSpace(content) == "" {
		return customerror.ErrInvalidInput
	}
	var postID int64
	err := s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		comment, err := tx.CommentRepository().SelectCommentByUUID(txCtx, commentUUID)
		if err != nil {
			return customerror.WrapRepository("select comment by uuid for update comment", err)
		}
		if comment == nil {
			return customerror.ErrCommentNotFound
		}
		requester, err := tx.UserRepository().SelectUserByID(txCtx, authorID)
		if err != nil {
			return customerror.WrapRepository("select user by id for update comment", err)
		}
		if requester == nil {
			return customerror.ErrUserNotFound
		}
		if err := s.authorizationPolicy.CanWrite(requester); err != nil {
			return err
		}
		if err := s.authorizationPolicy.OwnerOrAdmin(requester, comment.AuthorID); err != nil {
			return err
		}
		if err := s.ensureBoardVisibleByPostTx(tx, requester, comment.PostID); err != nil {
			return err
		}
		updatedComment := *comment
		updatedComment.Update(content)
		if err := tx.CommentRepository().Update(txCtx, &updatedComment); err != nil {
			return customerror.WrapRepository("update comment", err)
		}
		postID = updatedComment.PostID
		if err := dispatchDomainActions(tx, s.actionDispatcher, appevent.NewCommentChanged("updated", comment.ID, postID)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *CommentService) DeleteComment(ctx context.Context, commentUUID string, authorID int64) error {
	var commentID, postID int64
	if err := s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		comment, err := tx.CommentRepository().SelectCommentByUUID(txCtx, commentUUID)
		if err != nil {
			return customerror.WrapRepository("select comment by uuid for delete comment", err)
		}
		if comment == nil {
			return customerror.ErrCommentNotFound
		}
		requester, err := tx.UserRepository().SelectUserByID(txCtx, authorID)
		if err != nil {
			return customerror.WrapRepository("select user by id for delete comment", err)
		}
		if requester == nil {
			return customerror.ErrUserNotFound
		}
		if err := s.authorizationPolicy.CanWrite(requester); err != nil {
			return err
		}
		if err := s.authorizationPolicy.OwnerOrAdmin(requester, comment.AuthorID); err != nil {
			return err
		}
		if err := s.ensureBoardVisibleByPostTx(tx, requester, comment.PostID); err != nil {
			return err
		}
		if deleteErr := tx.CommentRepository().Delete(txCtx, comment.ID); deleteErr != nil {
			return customerror.WrapRepository("delete comment", deleteErr)
		}
		if _, reactionErr := tx.ReactionRepository().DeleteByTarget(txCtx, comment.ID, entity.ReactionTargetComment); reactionErr != nil {
			return customerror.WrapRepository("delete comment reactions", reactionErr)
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

func (s *CommentService) ensureBoardVisible(ctx context.Context, user *entity.User, boardID int64) error {
	board, err := s.boardRepository.SelectBoardByID(ctx, boardID)
	if err != nil {
		return customerror.WrapRepository("select board by id for comment board visibility", err)
	}
	return policy.EnsureBoardVisible(board, user)
}

func (s *CommentService) ensureBoardVisibleByPostTx(tx port.TxScope, user *entity.User, postID int64) error {
	post, err := tx.PostRepository().SelectPostByID(tx.Context(), postID)
	if err != nil {
		return customerror.WrapRepository("select post by id for comment board visibility", err)
	}
	if post == nil {
		return customerror.ErrPostNotFound
	}
	board, err := tx.BoardRepository().SelectBoardByID(tx.Context(), post.BoardID)
	if err != nil {
		return customerror.WrapRepository("select board by id for comment board visibility", err)
	}
	return policy.EnsureBoardVisible(board, user)
}
