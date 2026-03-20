package service

import (
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type postDeletionWorkflow struct {
	commentRepository     port.CommentRepository
	reactionRepository    port.ReactionRepository
	attachmentCoordinator *postAttachmentCoordinator
}

func newPostDeletionWorkflow(commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, attachmentCoordinator *postAttachmentCoordinator) *postDeletionWorkflow {
	return &postDeletionWorkflow{
		commentRepository:     commentRepository,
		reactionRepository:    reactionRepository,
		attachmentCoordinator: attachmentCoordinator,
	}
}

func (w *postDeletionWorkflow) deletePostArtifacts(tx port.TxScope, postID int64) ([]int64, error) {
	deletedCommentIDs, err := w.deletePostCommentsInBatches(tx, postID)
	if err != nil {
		return nil, err
	}
	if err := w.attachmentCoordinator.orphanPostAttachments(tx.Context(), tx.AttachmentRepository(), postID); err != nil {
		return nil, err
	}
	if _, reactionErr := tx.ReactionRepository().DeleteByTarget(tx.Context(), postID, entity.ReactionTargetPost); reactionErr != nil {
		return nil, customerror.WrapRepository("delete post reactions", reactionErr)
	}
	return deletedCommentIDs, nil
}

func (w *postDeletionWorkflow) deletePostCommentsInBatches(tx port.TxScope, postID int64) ([]int64, error) {
	txCtx := tx.Context()
	lastID := int64(0)
	deletedCommentIDs := make([]int64, 0)
	for {
		comments, err := tx.CommentRepository().SelectComments(txCtx, postID, postDeleteBatchSize, lastID)
		if err != nil {
			return nil, customerror.WrapRepository("select comments for delete post", err)
		}
		if len(comments) == 0 {
			return deletedCommentIDs, nil
		}
		for _, comment := range comments {
			deletedCommentIDs = append(deletedCommentIDs, comment.ID)
			if _, reactionErr := tx.ReactionRepository().DeleteByTarget(txCtx, comment.ID, entity.ReactionTargetComment); reactionErr != nil {
				return nil, customerror.WrapRepository("delete post comment reactions", reactionErr)
			}
			if deleteErr := tx.CommentRepository().Delete(txCtx, comment.ID); deleteErr != nil {
				return nil, customerror.WrapRepository("soft delete post comments", deleteErr)
			}
		}
		if len(comments) < postDeleteBatchSize {
			return deletedCommentIDs, nil
		}
		lastID = comments[len(comments)-1].ID
	}
}
