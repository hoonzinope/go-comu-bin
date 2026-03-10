package service

import "github.com/hoonzinope/go-comu-bin/internal/application/port"

func bestEffortCacheDelete(cache port.Cache, logger port.Logger, key, op string) {
	if err := cache.Delete(key); err != nil {
		logger.Warn("cache invalidation failed", "operation", op, "key", key, "error", err)
	}
}

func bestEffortCacheDeleteByPrefix(cache port.Cache, logger port.Logger, prefix, op string) {
	if _, err := cache.DeleteByPrefix(prefix); err != nil {
		logger.Warn("cache invalidation failed", "operation", op, "prefix", prefix, "error", err)
	}
}
