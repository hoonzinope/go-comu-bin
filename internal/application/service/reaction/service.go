package reaction

import (
	"context"
	"errors"
	"log/slog"

	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
	"github.com/hoonzinope/go-comu-bin/internal/application/cache/key"
	appevent "github.com/hoonzinope/go-comu-bin/internal/application/event"
	"github.com/hoonzinope/go-comu-bin/internal/application/mapper"
	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	svccommon "github.com/hoonzinope/go-comu-bin/internal/application/service/common"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.ReactionUseCase = (*Service)(nil)

type Service struct {
	queryHandler   *QueryHandler
	commandHandler *CommandHandler
}

func NewService(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, unitOfWork port.UnitOfWork, cache port.Cache, cachePolicy appcache.Policy, logger ...*slog.Logger) *Service {
	return NewServiceWithActionDispatcher(userRepository, boardRepository, postRepository, commentRepository, reactionRepository, unitOfWork, cache, nil, cachePolicy, logger...)
}

func NewServiceWithActionDispatcher(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, unitOfWork port.UnitOfWork, cache port.Cache, actionDispatcher port.ActionHookDispatcher, cachePolicy appcache.Policy, logger ...*slog.Logger) *Service {
	queryHandler := NewQueryHandler(userRepository, boardRepository, postRepository, commentRepository, reactionRepository, cache, cachePolicy)
	return &Service{
		queryHandler:   queryHandler,
		commandHandler: NewCommandHandler(userRepository, boardRepository, postRepository, commentRepository, reactionRepository, unitOfWork, svccommon.ResolveActionDispatcher(actionDispatcher), queryHandler),
	}
}

func (s *Service) SetReaction(ctx context.Context, userID int64, targetUUID string, targetType model.ReactionTargetType, reactionType model.ReactionType) (bool, error) {
	return s.commandHandler.SetReaction(ctx, userID, targetUUID, targetType, reactionType)
}

func (s *Service) DeleteReaction(ctx context.Context, userID int64, targetUUID string, targetType model.ReactionTargetType) error {
	return s.commandHandler.DeleteReaction(ctx, userID, targetUUID, targetType)
}

func (s *Service) GetReactionsByTarget(ctx context.Context, targetUUID string, targetType model.ReactionTargetType) ([]model.Reaction, error) {
	return s.queryHandler.GetReactionsByTarget(ctx, targetUUID, targetType)
}

type QueryHandler struct {
	userRepository     port.UserRepository
	boardRepository    port.BoardRepository
	postRepository     port.PostRepository
	commentRepository  port.CommentRepository
	reactionRepository port.ReactionRepository
	cache              port.Cache
	cachePolicy        appcache.Policy
}

func NewQueryHandler(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, cache port.Cache, cachePolicy appcache.Policy) *QueryHandler {
	return &QueryHandler{userRepository: userRepository, boardRepository: boardRepository, postRepository: postRepository, commentRepository: commentRepository, reactionRepository: reactionRepository, cache: cache, cachePolicy: cachePolicy}
}

func (h *QueryHandler) GetReactionsByTarget(ctx context.Context, targetUUID string, targetType model.ReactionTargetType) ([]model.Reaction, error) {
	entityTargetType, ok := targetType.ToEntity()
	if !ok {
		return nil, customerror.ErrInvalidInput
	}
	targetID, err := h.resolveTargetID(ctx, targetUUID, entityTargetType)
	if err != nil {
		return nil, err
	}
	cacheKey := key.ReactionList(string(entityTargetType), targetID)
	value, err := h.cache.GetOrSetWithTTL(ctx, cacheKey, h.cachePolicy.ListTTLSeconds, func(ctx context.Context) (interface{}, error) {
		if err := h.ensureTargetExists(ctx, nil, targetID, entityTargetType); err != nil {
			return nil, err
		}
		reactions, err := h.reactionRepository.GetByTarget(ctx, targetID, entityTargetType)
		if err != nil {
			return nil, customerror.WrapRepository("select reactions by target", err)
		}
		reactionModels, err := h.reactionsFromEntities(ctx, targetUUID, reactions)
		if err != nil {
			return nil, err
		}
		return reactionModels, nil
	})
	if err != nil {
		return nil, svccommon.NormalizeCacheLoadError("load reaction list cache", err)
	}
	reactions, ok := value.([]model.Reaction)
	if !ok {
		return nil, customerror.Mark(customerror.ErrCacheFailure, "decode reaction list cache payload")
	}
	return reactions, nil
}

func (h *QueryHandler) reactionsFromEntities(ctx context.Context, targetUUID string, reactions []*entity.Reaction) ([]model.Reaction, error) {
	userUUIDs, err := svccommon.UserUUIDsForReactions(ctx, h.userRepository, reactions)
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

func (h *QueryHandler) resolveTargetID(ctx context.Context, targetUUID string, targetType entity.ReactionTargetType) (int64, error) {
	switch targetType {
	case entity.ReactionTargetPost:
		post, err := h.postRepository.SelectPostByUUID(ctx, targetUUID)
		if err != nil {
			return 0, customerror.WrapRepository("select post by uuid for reaction target", err)
		}
		if post == nil {
			return 0, customerror.ErrPostNotFound
		}
		return post.ID, nil
	case entity.ReactionTargetComment:
		comment, err := h.commentRepository.SelectCommentByUUID(ctx, targetUUID)
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

func (h *QueryHandler) ensureTargetExists(ctx context.Context, user *entity.User, targetID int64, targetType entity.ReactionTargetType) error {
	switch targetType {
	case entity.ReactionTargetPost:
		_, err := policy.EnsurePostVisibleForUser(ctx, h.postRepository, h.boardRepository, user, targetID, customerror.ErrBoardNotFound, "reaction target")
		return err
	case entity.ReactionTargetComment:
		_, _, err := policy.EnsureCommentTargetVisibleForUser(ctx, h.commentRepository, h.postRepository, h.boardRepository, user, targetID, customerror.ErrBoardNotFound, "reaction target")
		return err
	default:
		return customerror.ErrInternalServerError
	}
}

type CommandHandler struct {
	unitOfWork          port.UnitOfWork
	actionDispatcher    port.ActionHookDispatcher
	queryHandler        *QueryHandler
	authorizationPolicy policy.AuthorizationPolicy
}

func NewCommandHandler(_ port.UserRepository, _ port.BoardRepository, _ port.PostRepository, _ port.CommentRepository, _ port.ReactionRepository, unitOfWork port.UnitOfWork, actionDispatcher port.ActionHookDispatcher, queryHandler *QueryHandler) *CommandHandler {
	return &CommandHandler{unitOfWork: unitOfWork, actionDispatcher: actionDispatcher, queryHandler: queryHandler, authorizationPolicy: policy.NewRoleAuthorizationPolicy()}
}

func (h *CommandHandler) SetReaction(ctx context.Context, userID int64, targetUUID string, targetType model.ReactionTargetType, reactionType model.ReactionType) (bool, error) {
	entityTargetType, entityReactionType, err := reactionInputTypes(targetType, reactionType)
	if err != nil {
		return false, err
	}
	targetID, err := h.queryHandler.resolveTargetID(ctx, targetUUID, entityTargetType)
	if err != nil {
		return false, err
	}
	created, changed, err := h.withReactionTransaction(ctx, userID, targetID, entityTargetType, func(tx port.TxScope, detailPostID *int64) (bool, bool, error) {
		_, created, changed, err := tx.ReactionRepository().SetUserTargetReaction(tx.Context(), userID, targetID, entityTargetType, entityReactionType)
		if err != nil {
			return false, false, customerror.WrapRepository("set user target reaction", err)
		}
		if (created || changed) && detailPostID != nil {
			if err := svccommon.DispatchDomainActions(tx, h.actionDispatcher, appevent.NewReactionChanged("set", entityTargetType, targetID, *detailPostID, userID, entityReactionType)); err != nil {
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

func (h *CommandHandler) DeleteReaction(ctx context.Context, userID int64, targetUUID string, targetType model.ReactionTargetType) error {
	entityTargetType, ok := targetType.ToEntity()
	if !ok {
		return customerror.ErrInvalidInput
	}
	targetID, err := h.queryHandler.resolveTargetID(ctx, targetUUID, entityTargetType)
	if err != nil {
		return err
	}
	_, _, err = h.withReactionTransaction(ctx, userID, targetID, entityTargetType, func(tx port.TxScope, detailPostID *int64) (bool, bool, error) {
		existing, err := tx.ReactionRepository().GetUserTargetReaction(tx.Context(), userID, targetID, entityTargetType)
		if err != nil {
			return false, false, customerror.WrapRepository("select user target reaction", err)
		}
		deleted, err := tx.ReactionRepository().DeleteUserTargetReaction(tx.Context(), userID, targetID, entityTargetType)
		if err != nil {
			return false, false, customerror.WrapRepository("delete user target reaction", err)
		}
		if deleted && detailPostID != nil && existing != nil {
			if err := svccommon.DispatchDomainActions(tx, h.actionDispatcher, appevent.NewReactionChanged("unset", entityTargetType, targetID, *detailPostID, userID, existing.Type)); err != nil {
				return false, false, err
			}
		}
		return deleted, deleted, nil
	})
	if err != nil {
		return err
	}
	return nil
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

func (h *CommandHandler) withReactionTransaction(ctx context.Context, userID, targetID int64, targetType entity.ReactionTargetType, mutate func(tx port.TxScope, detailPostID *int64) (bool, bool, error)) (bool, bool, error) {
	var created bool
	var changed bool
	err := h.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		user, err := tx.UserRepository().SelectUserByID(tx.Context(), userID)
		if err != nil {
			return customerror.WrapRepository("select user by id for reaction", err)
		}
		if user == nil {
			return customerror.ErrUserNotFound
		}
		if err := policy.ForbidGuest(user); err != nil {
			return err
		}
		if err := h.authorizationPolicy.CanWrite(user); err != nil {
			return err
		}
		postID, err := h.ensureTargetExistsTx(tx, user, targetID, targetType)
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

func (h *CommandHandler) ensureTargetExistsTx(tx port.TxScope, user *entity.User, targetID int64, targetType entity.ReactionTargetType) (*int64, error) {
	txCtx := tx.Context()
	switch targetType {
	case entity.ReactionTargetPost:
		post, err := policy.EnsurePostVisibleForUser(txCtx, tx.PostRepository(), tx.BoardRepository(), user, targetID, customerror.ErrBoardNotFound, "reaction target")
		if err != nil {
			return nil, err
		}
		postID := post.ID
		return &postID, nil
	case entity.ReactionTargetComment:
		comment, _, err := policy.EnsureCommentTargetVisibleForUser(txCtx, tx.CommentRepository(), tx.PostRepository(), tx.BoardRepository(), user, targetID, customerror.ErrBoardNotFound, "reaction target")
		if err != nil {
			return nil, err
		}
		postID := comment.PostID
		return &postID, nil
	default:
		return nil, customerror.ErrInternalServerError
	}
}
