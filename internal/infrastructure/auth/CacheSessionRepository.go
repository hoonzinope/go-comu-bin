package auth

import (
	"fmt"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
)

var _ port.SessionRepository = (*CacheSessionRepository)(nil)

type CacheSessionRepository struct {
	cache port.Cache
}

func NewCacheSessionRepository(cache port.Cache) *CacheSessionRepository {
	return &CacheSessionRepository{cache: cache}
}

func (r *CacheSessionRepository) Save(userID int64, token string, ttlSeconds int) error {
	r.cache.SetWithTTL(sessionCacheKey(userID, token), userID, ttlSeconds)
	return nil
}

func (r *CacheSessionRepository) Delete(userID int64, token string) error {
	r.cache.Delete(sessionCacheKey(userID, token))
	return nil
}

func (r *CacheSessionRepository) DeleteByUser(userID int64) error {
	r.cache.DeleteByPrefix(sessionCachePrefix(userID))
	return nil
}

func (r *CacheSessionRepository) Exists(userID int64, token string) (bool, error) {
	_, exists := r.cache.Get(sessionCacheKey(userID, token))
	return exists, nil
}

func sessionCachePrefix(userID int64) string {
	return fmt.Sprintf("session:user:%d:", userID)
}

func sessionCacheKey(userID int64, token string) string {
	return sessionCachePrefix(userID) + token
}
