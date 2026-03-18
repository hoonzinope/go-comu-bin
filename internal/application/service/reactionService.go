package service

import (
	"context"
	"errors"
	"log/slog"

	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
	"github.com/hoonzinope/go-comu-bin/internal/application/cache/key"
	appevent "github.com/hoonzinope/go-comu-bin/internal/application/event"
	"github.com/hoonzinope/go-comu-bin/internal/application/mapper"
	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.ReactionUseCase = (*ReactionService)(nil)

type ReactionService struct {
	userRepository     port.UserRepository
	boardRepository    port.BoardRepository
	postRepository     port.PostRepository
	commentRepository  port.CommentRepository
	reactionRepository port.ReactionRepository
	unitOfWork         port.UnitOfWork
	cache              port.Cache
	actionDispatcher   port.ActionHookDispatcher
	cachePolicy        appcache.Policy
	logger             *slog.Logger
}

func NewReactionService(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, unitOfWork port.UnitOfWork, cache port.Cache, cachePolicy appcache.Policy, logger ...*slog.Logger) *ReactionService {
	return NewReactionServiceWithActionDispatcher(userRepository, boardRepository, postRepository, commentRepository, reactionRepository, unitOfWork, cache, nil, cachePolicy, logger...)
}

func NewReactionServiceWithActionDispatcher(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, unitOfWork port.UnitOfWork, cache port.Cache, actionDispatcher port.ActionHookDispatcher, cachePolicy appcache.Policy, logger ...*slog.Logger) *ReactionService {
	return &ReactionService{
		userRepository:     userRepository,
		boardRepository:    boardRepository,
		postRepository:     postRepository,
		commentRepository:  commentRepository,
		reactionRepository: reactionRepository,
		unitOfWork:         unitOfWork,
		cache:              cache,
		actionDispatcher:   resolveActionDispatcher(actionDispatcher),
		cachePolicy:        cachePolicy,
		logger:             resolveLogger(logger),
	}
}

func (s *ReactionService) SetReaction(ctx context.Context, userID int64, targetUUID string, targetType model.ReactionTargetType, reactionType model.ReactionType) (bool, error) {
	entityTargetType, entityReactionType, err := reactionInputTypes(targetType, reactionType)
	if err != nil {
		return false, err
	}
	targetID, err := s.resolveTargetID(ctx, targetUUID, entityTargetType)
	if err != nil {
		return false, err
	}
	created, changed, err := s.withReactionTransaction(ctx, userID, targetID, entityTargetType, func(tx port.TxScope, detailPostID *int64) (bool, bool, error) {
		_, created, changed, err := tx.ReactionRepository().SetUserTargetReaction(tx.Context(), userID, targetID, entityTargetType, entityReactionType)
		if err != nil {
			return false, false, customerror.WrapRepository("set user target reaction", err)
		}
		if (created || changed) && detailPostID != nil {
			if err := dispatchDomainActions(tx, s.actionDispatcher, appevent.NewReactionChanged("set", entityTargetType, targetID, *detailPostID)); err != nil {
				return false, false, err
			}
		}
		return created, changed, nil
	})
	if err != nil {
		return false, err
	}
	if !created && !changed {
		return false, nil
	}
	return created, nil
}

func (s *ReactionService) DeleteReaction(ctx context.Context, userID int64, targetUUID string, targetType model.ReactionTargetType) error {
	entityTargetType, ok := targetType.ToEntity()
	if !ok {
		return customerror.ErrInvalidInput
	}
	targetID, err := s.resolveTargetID(ctx, targetUUID, entityTargetType)
	if err != nil {
		return err
	}
	deleted, _, err := s.withReactionTransaction(ctx, userID, targetID, entityTargetType, func(tx port.TxScope, detailPostID *int64) (bool, bool, error) {
		deleted, err := tx.ReactionRepository().DeleteUserTargetReaction(tx.Context(), userID, targetID, entityTargetType)
		if err != nil {
			return false, false, customerror.WrapRepository("delete user target reaction", err)
		}
		if deleted && detailPostID != nil {
			if err := dispatchDomainActions(tx, s.actionDispatcher, appevent.NewReactionChanged("unset", entityTargetType, targetID, *detailPostID)); err != nil {
				return false, false, err
			}
		}
		return deleted, deleted, nil
	})
	if err != nil {
		return err
	}
	if !deleted {
		return nil
	}
	return nil
}

func (s *ReactionService) GetReactionsByTarget(ctx context.Context, targetUUID string, targetType model.ReactionTargetType) ([]model.Reaction, error) {
	entityTargetType, ok := targetType.ToEntity()
	if !ok {
		return nil, customerror.ErrInvalidInput
	}
	targetID, err := s.resolveTargetID(ctx, targetUUID, entityTargetType)
	if err != nil {
		return nil, err
	}
	cacheKey := key.ReactionList(string(entityTargetType), targetID)
	value, err := s.cache.GetOrSetWithTTL(ctx, cacheKey, s.cachePolicy.ListTTLSeconds, func(ctx context.Context) (interface{}, error) {
		if err := s.ensureTargetExists(ctx, nil, targetID, entityTargetType); err != nil {
			return nil, err
		}
		reactions, err := s.reactionRepository.GetByTarget(ctx, targetID, entityTargetType)
		if err != nil {
			return nil, customerror.WrapRepository("select reactions by target", err)
		}
		reactionModels, err := s.reactionsFromEntities(ctx, targetUUID, reactions)
		if err != nil {
			return nil, err
		}
		return reactionModels, nil
	})
	if err != nil {
		return nil, normalizeCacheLoadError("load reaction list cache", err)
	}
	reactions, ok := value.([]model.Reaction)
	if !ok {
		return nil, customerror.Mark(customerror.ErrCacheFailure, "decode reaction list cache payload")
	}
	return reactions, nil
}

func reactionInputTypes(targetType model.ReactionTargetType, reactionType model.ReactionType) (entity.ReactionTargetType, entity.ReactionType, error) {
	entityTargetType, ok := targetType.ToEntity()
	if !ok {
		return "", "", customerror.ErrInvalidInput
	}
	entityReactionType, ok := reactionType.ToEntity()
	if !ok {
		return "", "", customerror.ErrInvalidInput
	}
	return entityTargetType, entityReactionType, nil
}

func (s *ReactionService) withReactionTransaction(ctx context.Context, userID, targetID int64, targetType entity.ReactionTargetType, mutate func(tx port.TxScope, detailPostID *int64) (bool, bool, error)) (bool, bool, error) {
	var created bool
	var changed bool
	err := s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		user, err := tx.UserRepository().SelectUserByID(tx.Context(), userID)
		if err != nil {
			return customerror.WrapRepository("select user by id for reaction", err)
		}
		if user == nil {
			return customerror.ErrUserNotFound
		}
		postID, err := s.ensureTargetExistsTx(tx, user, targetID, targetType)
		if err != nil {
			return err
		}
		created, changed, err = mutate(tx, postID)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return false, false, err
	}
	return created, changed, nil
}

func (s *ReactionService) reactionsFromEntities(ctx context.Context, targetUUID string, reactions []*entity.Reaction) ([]model.Reaction, error) {
	userUUIDs, err := userUUIDsForReactions(ctx, s.userRepository, reactions)
	if err != nil {
		return nil, err
	}
	out := make([]model.Reaction, 0, len(reactions))
	for _, reaction := range reactions {
		userUUID, ok := userUUIDs[reaction.UserID]
		if !ok {
			return nil, customerror.WrapRepository("select users by ids including deleted", errors.New("reaction user not found"))
		}
		reactionModel := mapper.ReactionFromEntity(reaction)
		reactionModel.TargetUUID = targetUUID
		reactionModel.UserUUID = userUUID
		out = append(out, reactionModel)
	}
	return out, nil
}

func (s *ReactionService) resolveTargetID(ctx context.Context, targetUUID string, targetType entity.ReactionTargetType) (int64, error) {
	switch targetType {
	case entity.ReactionTargetPost:
		post, err := s.postRepository.SelectPostByUUID(ctx, targetUUID)
		if err != nil {
			return 0, customerror.WrapRepository("select post by uuid for reaction target", err)
		}
		if post == nil {
			return 0, customerror.ErrPostNotFound
		}
		return post.ID, nil
	case entity.ReactionTargetComment:
		comment, err := s.commentRepository.SelectCommentByUUID(ctx, targetUUID)
		if err != nil {
			return 0, customerror.WrapRepository("select comment by uuid for reaction target", err)
		}
		if comment == nil {
			return 0, customerror.ErrCommentNotFound
		}
		return comment.ID, nil
	default:
		return 0, customerror.ErrInvalidInput
	}
}

func (s *ReactionService) ensureTargetExistsTx(tx port.TxScope, user *entity.User, targetID int64, targetType entity.ReactionTargetType) (*int64, error) {
	txCtx := tx.Context()
	switch targetType {
	case entity.ReactionTargetPost:
		post, err := ensurePostVisibleForUser(txCtx, tx.PostRepository(), tx.BoardRepository(), user, targetID, customerror.ErrBoardNotFound, "reaction target")
		if err != nil {
			return nil, err
		}
		postID := post.ID
		return &postID, nil
	case entity.ReactionTargetComment:
		comment, _, err := ensureCommentTargetVisibleForUser(txCtx, tx.CommentRepository(), tx.PostRepository(), tx.BoardRepository(), user, targetID, customerror.ErrBoardNotFound, "reaction target")
		if err != nil {
			return nil, err
		}
		postID := comment.PostID
		return &postID, nil
	default:
		return nil, customerror.ErrInternalServerError
	}
}

func (s *ReactionService) ensureTargetExists(ctx context.Context, user *entity.User, targetID int64, targetType entity.ReactionTargetType) error {
	switch targetType {
	case entity.ReactionTargetPost:
		_, err := ensurePostVisibleForUser(ctx, s.postRepository, s.boardRepository, user, targetID, customerror.ErrBoardNotFound, "reaction target")
		return err
	case entity.ReactionTargetComment:
		_, _, err := ensureCommentTargetVisibleForUser(ctx, s.commentRepository, s.postRepository, s.boardRepository, user, targetID, customerror.ErrBoardNotFound, "reaction target")
		return err
	default:
		return customerror.ErrInternalServerError
	}
}
