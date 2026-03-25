package event

import (
	"context"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type PostRankingHandler struct {
	repository port.PostRankingRepository
}

func NewPostRankingHandler(repository port.PostRankingRepository) *PostRankingHandler {
	return &PostRankingHandler{repository: repository}
}

func (h *PostRankingHandler) Handle(ctx context.Context, event port.DomainEvent) error {
	if h == nil || h.repository == nil || event == nil {
		return nil
	}
	switch e := event.(type) {
	case PostChanged:
		if e.Operation == "deleted" {
			return h.repository.DeletePost(ctx, e.PostID)
		}
		return h.repository.UpsertPostSnapshot(ctx, e.PostID, e.BoardID, e.PublishedAt, entity.PostStatusPublished)
	case CommentChanged:
		switch e.Operation {
		case "created":
			return h.repository.ApplyCommentDelta(ctx, e.PostID, e.CommentID, e.At, 2)
		case "deleted":
			return h.repository.ApplyCommentDelta(ctx, e.PostID, e.CommentID, e.At, -2)
		default:
			return nil
		}
	case ReactionChanged:
		return h.repository.ApplyReactionDelta(ctx, e.PostID, e.TargetID, e.UserID, e.TargetType, e.ReactionType, e.Operation, e.At)
	default:
		return nil
	}
}
