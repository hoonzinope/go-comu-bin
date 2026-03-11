package event

import (
	"github.com/hoonzinope/go-comu-bin/internal/application/cache/key"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.EventHandler = (*CacheInvalidationHandler)(nil)

type CacheInvalidationHandler struct {
	cache  port.Cache
	logger port.Logger
}

func NewCacheInvalidationHandler(cache port.Cache, logger port.Logger) *CacheInvalidationHandler {
	return &CacheInvalidationHandler{cache: cache, logger: logger}
}

func (h *CacheInvalidationHandler) Handle(event port.DomainEvent) error {
	switch e := event.(type) {
	case BoardChanged:
		h.bestEffortDeleteByPrefix(key.BoardListPrefix(), "invalidate board list by event")
	case PostChanged:
		h.handlePostChanged(e)
	case CommentChanged:
		h.handleCommentChanged(e)
	case ReactionChanged:
		h.handleReactionChanged(e)
	case AttachmentChanged:
		h.bestEffortDelete(key.PostDetail(e.PostID), "invalidate post detail by attachment event")
	}
	return nil
}

func (h *CacheInvalidationHandler) handlePostChanged(e PostChanged) {
	h.bestEffortDeleteByPrefix(key.PostListPrefix(e.BoardID), "invalidate post list by event")
	h.bestEffortDelete(key.PostDetail(e.PostID), "invalidate post detail by event")
	for _, tagName := range e.TagNames {
		h.bestEffortDeleteByPrefix(key.TagPostListPrefix(tagName), "invalidate tag post list by event")
	}
	if e.Operation != "deleted" {
		return
	}
	h.bestEffortDeleteByPrefix(key.CommentListPrefix(e.PostID), "invalidate comment list by post delete event")
	h.bestEffortDelete(key.ReactionList(string(entity.ReactionTargetPost), e.PostID), "invalidate post reaction list by post delete event")
	for _, commentID := range e.DeletedCommentIDs {
		h.bestEffortDelete(key.ReactionList(string(entity.ReactionTargetComment), commentID), "invalidate comment reaction list by post delete event")
	}
}

func (h *CacheInvalidationHandler) handleCommentChanged(e CommentChanged) {
	h.bestEffortDeleteByPrefix(key.CommentListPrefix(e.PostID), "invalidate comment list by event")
	h.bestEffortDelete(key.PostDetail(e.PostID), "invalidate post detail by comment event")
	if e.Operation == "deleted" {
		h.bestEffortDelete(key.ReactionList(string(entity.ReactionTargetComment), e.CommentID), "invalidate comment reaction list by comment delete event")
	}
}

func (h *CacheInvalidationHandler) handleReactionChanged(e ReactionChanged) {
	h.bestEffortDelete(key.ReactionList(string(e.TargetType), e.TargetID), "invalidate reaction list by event")
	h.bestEffortDelete(key.PostDetail(e.PostID), "invalidate post detail by reaction event")
}

func (h *CacheInvalidationHandler) bestEffortDelete(cacheKey, operation string) {
	if err := h.cache.Delete(cacheKey); err != nil {
		h.logger.Warn("cache invalidation failed", "operation", operation, "key", cacheKey, "error", err)
	}
}

func (h *CacheInvalidationHandler) bestEffortDeleteByPrefix(prefix, operation string) {
	if _, err := h.cache.DeleteByPrefix(prefix); err != nil {
		h.logger.Warn("cache invalidation failed", "operation", operation, "prefix", prefix, "error", err)
	}
}
