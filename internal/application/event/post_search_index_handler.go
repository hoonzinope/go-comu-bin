package event

import (
	"context"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
)

var _ port.EventHandler = (*PostSearchIndexHandler)(nil)

type PostSearchIndexHandler struct {
	indexer port.PostSearchIndexer
}

func NewPostSearchIndexHandler(indexer port.PostSearchIndexer) *PostSearchIndexHandler {
	return &PostSearchIndexHandler{indexer: indexer}
}

func (h *PostSearchIndexHandler) Handle(ctx context.Context, event port.DomainEvent) error {
	if h == nil || h.indexer == nil || event == nil {
		return nil
	}
	postChanged, ok := event.(PostChanged)
	if !ok {
		return nil
	}
	switch postChanged.Operation {
	case "deleted":
		return h.indexer.DeletePost(ctx, postChanged.PostID)
	default:
		return h.indexer.UpsertPost(ctx, postChanged.PostID)
	}
}
