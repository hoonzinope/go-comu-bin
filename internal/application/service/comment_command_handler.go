package service

import (
	"context"
	"strings"

	appevent "github.com/hoonzinope/go-comu-bin/internal/application/event"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type commentCommandHandler struct {
	unitOfWork          port.UnitOfWork
	actionDispatcher    port.ActionHookDispatcher
	authorizationPolicy policy.AuthorizationPolicy
}

func newCommentCommandHandler(unitOfWork port.UnitOfWork, actionDispatcher port.ActionHookDispatcher, authorizationPolicy policy.AuthorizationPolicy) *commentCommandHandler {
	return &commentCommandHandler{unitOfWork: unitOfWork, actionDispatcher: actionDispatcher, authorizationPolicy: authorizationPolicy}
}

func (h *commentCommandHandler) CreateComment(ctx context.Context, content string, authorID int64, postUUID string, parentUUID *string) (string, error) {
	if strings.TrimSpace(content) == "" {
		return "", customerror.ErrInvalidInput
	}
	var commentUUID string
	err := h.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		user, err := tx.UserRepository().SelectUserByID(txCtx, authorID)
		if err != nil {
			return customerror.WrapRepository("select user by id for create comment", err)
		}
		if user == nil {
			return customerror.ErrUserNotFound
		}
		if err := policy.EnsureGuestLifecycleAllowsWrite(user); err != nil {
			return err
		}
		if err := h.authorizationPolicy.CanWrite(user); err != nil {
			return err
		}
		post, err := tx.PostRepository().SelectPostByUUID(txCtx, postUUID)
		if err != nil {
			return customerror.WrapRepository("select post by uuid for create comment", err)
		}
		if post == nil {
			return customerror.ErrPostNotFound
		}
		if err := ensureCommentBoardVisibleByPostTx(tx, user, post.ID); err != nil {
			return err
		}
		newComment := entity.NewComment(content, authorID, post.ID, nil)
		if parentUUID != nil && strings.TrimSpace(*parentUUID) != "" {
			parent, err := tx.CommentRepository().SelectCommentByUUID(txCtx, strings.TrimSpace(*parentUUID))
			if err != nil {
				return customerror.WrapRepository("select parent comment by uuid for create comment", err)
			}
			if parent == nil {
				return customerror.ErrCommentNotFound
			}
			if parent.PostID != post.ID || parent.ParentID != nil || parent.Status != entity.CommentStatusActive {
				return customerror.ErrInvalidInput
			}
			newComment.ParentID = &parent.ID
		}
		commentID, err := tx.CommentRepository().Save(txCtx, newComment)
		if err != nil {
			return customerror.WrapRepository("save comment", err)
		}
		commentUUID = newComment.UUID
		if err := dispatchDomainActions(tx, h.actionDispatcher, appevent.NewCommentChanged("created", commentID, post.ID)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return commentUUID, nil
}

func (h *commentCommandHandler) UpdateComment(ctx context.Context, commentUUID string, authorID int64, content string) error {
	if strings.TrimSpace(content) == "" {
		return customerror.ErrInvalidInput
	}
	return h.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
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
		if err := policy.EnsureGuestLifecycleAllowsWrite(requester); err != nil {
			return err
		}
		if err := h.authorizationPolicy.CanWrite(requester); err != nil {
			return err
		}
		if err := h.authorizationPolicy.OwnerOrAdmin(requester, comment.AuthorID); err != nil {
			return err
		}
		if err := ensureCommentBoardVisibleByPostTx(tx, requester, comment.PostID); err != nil {
			return err
		}
		updatedComment := *comment
		updatedComment.Update(content)
		if err := tx.CommentRepository().Update(txCtx, &updatedComment); err != nil {
			return customerror.WrapRepository("update comment", err)
		}
		return dispatchDomainActions(tx, h.actionDispatcher, appevent.NewCommentChanged("updated", comment.ID, updatedComment.PostID))
	})
}

func (h *commentCommandHandler) DeleteComment(ctx context.Context, commentUUID string, authorID int64) error {
	return h.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
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
		if err := policy.EnsureGuestLifecycleAllowsWrite(requester); err != nil {
			return err
		}
		if err := h.authorizationPolicy.CanWrite(requester); err != nil {
			return err
		}
		if err := h.authorizationPolicy.OwnerOrAdmin(requester, comment.AuthorID); err != nil {
			return err
		}
		if err := ensureCommentBoardVisibleByPostTx(tx, requester, comment.PostID); err != nil {
			return err
		}
		if deleteErr := tx.CommentRepository().Delete(txCtx, comment.ID); deleteErr != nil {
			return customerror.WrapRepository("delete comment", deleteErr)
		}
		if _, reactionErr := tx.ReactionRepository().DeleteByTarget(txCtx, comment.ID, entity.ReactionTargetComment); reactionErr != nil {
			return customerror.WrapRepository("delete comment reactions", reactionErr)
		}
		return dispatchDomainActions(tx, h.actionDispatcher, appevent.NewCommentChanged("deleted", comment.ID, comment.PostID))
	})
}

func ensureCommentBoardVisibleByPostTx(tx port.TxScope, user *entity.User, postID int64) error {
	_, err := policy.EnsurePostVisibleForUser(tx.Context(), tx.PostRepository(), tx.BoardRepository(), user, postID, customerror.ErrBoardNotFound, "comment board visibility")
	return err
}
