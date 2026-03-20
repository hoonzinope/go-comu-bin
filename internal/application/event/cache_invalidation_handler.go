package event

import (
	"context"
	"log/slog"

	"github.com/hoonzinope/go-comu-bin/internal/application/cache/key"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.EventHandler = (*CacheInvalidationHandler)(nil)

type CacheInvalidationHandler struct {
	cache  port.Cache
	logger *slog.Logger
}

func NewCacheInvalidationHandler(cache port.Cache, logger *slog.Logger) *CacheInvalidationHandler {
	return &CacheInvalidationHandler{cache: cache, logger: logger}
}

func (h *CacheInvalidationHandler) Handle(ctx context.Context, event port.DomainEvent) error {
	switch e := event.(type) {
	case BoardChanged:
		h.handleBoardChanged(ctx, e)
	case PostChanged:
		h.handlePostChanged(ctx, e)
	case CommentChanged:
		h.handleCommentChanged(ctx, e)
	case ReactionChanged:
		h.handleReactionChanged(ctx, e)
	case AttachmentChanged:
		h.bestEffortDelete(ctx, key.PostDetail(e.PostID), "invalidate post detail by attachment event")
	}
	return nil
}

func (h *CacheInvalidationHandler) handleBoardChanged(ctx context.Context, e BoardChanged) {
	h.bestEffortDeleteByPrefix(ctx, key.BoardListPrefix(), "invalidate board list by event")
	h.bestEffortDeleteByPrefix(ctx, key.PostListPrefix(e.BoardID), "invalidate post list by board event")
	h.bestEffortDeleteByPrefix(ctx, key.PostSearchListPrefix(), "invalidate post search list by board event")
	if e.Operation != "visibility" {
		return
	}
	// Visibility changes can conceal/expose many cached resources.
	h.bestEffortDeleteByPrefix(ctx, key.PostDetailPrefix(), "invalidate post detail by board visibility event")
	h.bestEffortDeleteByPrefix(ctx, key.CommentListGlobalPrefix(), "invalidate comment list by board visibility event")
	h.bestEffortDeleteByPrefix(ctx, key.ReactionListPrefix(), "invalidate reaction list by board visibility event")
	h.bestEffortDeleteByPrefix(ctx, key.TagPostListGlobalPrefix(), "invalidate tag post list by board visibility event")
}

func (h *CacheInvalidationHandler) handlePostChanged(ctx context.Context, e PostChanged) {
	h.bestEffortDeleteByPrefix(ctx, key.PostListPrefix(e.BoardID), "invalidate post list by event")
	h.bestEffortDeleteByPrefix(ctx, key.PostSearchListPrefix(), "invalidate post search list by event")
	h.bestEffortDelete(ctx, key.PostDetail(e.PostID), "invalidate post detail by event")
	for _, tagName := range e.TagNames {
		h.bestEffortDeleteByPrefix(ctx, key.TagPostListPrefix(tagName), "invalidate tag post list by event")
	}
	if e.Operation != "deleted" {
		return
	}
	h.bestEffortDeleteByPrefix(ctx, key.CommentListPrefix(e.PostID), "invalidate comment list by post delete event")
	h.bestEffortDelete(ctx, key.ReactionList(string(entity.ReactionTargetPost), e.PostID), "invalidate post reaction list by post delete event")
	for _, commentID := range e.DeletedCommentIDs {
		h.bestEffortDelete(ctx, key.ReactionList(string(entity.ReactionTargetComment), commentID), "invalidate comment reaction list by post delete event")
	}
}

func (h *CacheInvalidationHandler) handleCommentChanged(ctx context.Context, e CommentChanged) {
	h.bestEffortDeleteByPrefix(ctx, key.CommentListPrefix(e.PostID), "invalidate comment list by event")
	h.bestEffortDelete(ctx, key.PostDetail(e.PostID), "invalidate post detail by comment event")
	if e.Operation == "deleted" {
		h.bestEffortDelete(ctx, key.ReactionList(string(entity.ReactionTargetComment), e.CommentID), "invalidate comment reaction list by comment delete event")
	}
}

func (h *CacheInvalidationHandler) handleReactionChanged(ctx context.Context, e ReactionChanged) {
	h.bestEffortDelete(ctx, key.ReactionList(string(e.TargetType), e.TargetID), "invalidate reaction list by event")
	h.bestEffortDelete(ctx, key.PostDetail(e.PostID), "invalidate post detail by reaction event")
}

func (h *CacheInvalidationHandler) bestEffortDelete(ctx context.Context, cacheKey, operation string) {
	if err := h.cache.Delete(ctx, cacheKey); err != nil {
		h.logger.Warn("cache invalidation failed", "operation", operation, "key", cacheKey, "error", err)
	}
}

func (h *CacheInvalidationHandler) bestEffortDeleteByPrefix(ctx context.Context, prefix, operation string) {
	if _, err := h.cache.DeleteByPrefix(ctx, prefix); err != nil {
		h.logger.Warn("cache invalidation failed", "operation", operation, "prefix", prefix, "error", err)
	}
}
