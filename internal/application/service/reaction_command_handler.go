package service

import (
	"context"

	appevent "github.com/hoonzinope/go-comu-bin/internal/application/event"
	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type reactionCommandHandler struct {
	userRepository     port.UserRepository
	boardRepository    port.BoardRepository
	postRepository     port.PostRepository
	commentRepository  port.CommentRepository
	reactionRepository port.ReactionRepository
	unitOfWork         port.UnitOfWork
	actionDispatcher   port.ActionHookDispatcher
	queryHandler       *reactionQueryHandler
}

func newReactionCommandHandler(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, unitOfWork port.UnitOfWork, actionDispatcher port.ActionHookDispatcher, queryHandler *reactionQueryHandler) *reactionCommandHandler {
	return &reactionCommandHandler{userRepository: userRepository, boardRepository: boardRepository, postRepository: postRepository, commentRepository: commentRepository, reactionRepository: reactionRepository, unitOfWork: unitOfWork, actionDispatcher: actionDispatcher, queryHandler: queryHandler}
}

func (h *reactionCommandHandler) SetReaction(ctx context.Context, userID int64, targetUUID string, targetType model.ReactionTargetType, reactionType model.ReactionType) (bool, error) {
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
			if err := dispatchDomainActions(tx, h.actionDispatcher, appevent.NewReactionChanged("set", entityTargetType, targetID, *detailPostID)); err != nil {
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

func (h *reactionCommandHandler) DeleteReaction(ctx context.Context, userID int64, targetUUID string, targetType model.ReactionTargetType) error {
	entityTargetType, ok := targetType.ToEntity()
	if !ok {
		return customerror.ErrInvalidInput
	}
	targetID, err := h.queryHandler.resolveTargetID(ctx, targetUUID, entityTargetType)
	if err != nil {
		return err
	}
	deleted, _, err := h.withReactionTransaction(ctx, userID, targetID, entityTargetType, func(tx port.TxScope, detailPostID *int64) (bool, bool, error) {
		deleted, err := tx.ReactionRepository().DeleteUserTargetReaction(tx.Context(), userID, targetID, entityTargetType)
		if err != nil {
			return false, false, customerror.WrapRepository("delete user target reaction", err)
		}
		if deleted && detailPostID != nil {
			if err := dispatchDomainActions(tx, h.actionDispatcher, appevent.NewReactionChanged("unset", entityTargetType, targetID, *detailPostID)); err != nil {
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

func (h *reactionCommandHandler) withReactionTransaction(ctx context.Context, userID, targetID int64, targetType entity.ReactionTargetType, mutate func(tx port.TxScope, detailPostID *int64) (bool, bool, error)) (bool, bool, error) {
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

func (h *reactionCommandHandler) ensureTargetExistsTx(tx port.TxScope, user *entity.User, targetID int64, targetType entity.ReactionTargetType) (*int64, error) {
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
