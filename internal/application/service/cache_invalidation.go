package service

import (
	"log/slog"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
)

func bestEffortCacheDelete(cache port.Cache, key, op string) {
	if err := cache.Delete(key); err != nil {
		slog.Warn("cache invalidation failed", "operation", op, "key", key, "error", err)
	}
}

func bestEffortCacheDeleteByPrefix(cache port.Cache, prefix, op string) {
	if _, err := cache.DeleteByPrefix(prefix); err != nil {
		slog.Warn("cache invalidation failed", "operation", op, "prefix", prefix, "error", err)
	}
}
