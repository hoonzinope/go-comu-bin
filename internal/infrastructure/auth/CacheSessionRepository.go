package auth

import (
	"context"
	"fmt"
	"sync"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
)

var _ port.SessionRepository = (*CacheSessionRepository)(nil)

type CacheSessionRepository struct {
	cache     port.Cache
	userLocks sync.Map
}

type sessionRepositoryScope struct {
	repo *CacheSessionRepository
}

func NewCacheSessionRepository(cache port.Cache) *CacheSessionRepository {
	return &CacheSessionRepository{cache: cache}
}

func (r *CacheSessionRepository) Save(ctx context.Context, userID int64, token string, ttlSeconds int) error {
	return r.withUserLock(userID, func() error {
		return r.cache.SetWithTTL(ctx, sessionCacheKey(userID, token), userID, ttlSeconds)
	})
}

func (r *CacheSessionRepository) Delete(ctx context.Context, userID int64, token string) error {
	return r.withUserLock(userID, func() error {
		return r.cache.Delete(ctx, sessionCacheKey(userID, token))
	})
}

func (r *CacheSessionRepository) DeleteByUser(ctx context.Context, userID int64) error {
	return r.withUserLock(userID, func() error {
		_, err := r.cache.DeleteByPrefix(ctx, sessionCachePrefix(userID))
		return err
	})
}

func (r *CacheSessionRepository) Exists(ctx context.Context, userID int64, token string) (bool, error) {
	var (
		exists bool
		err    error
	)
	if err = r.withUserLock(userID, func() error {
		exists, err = r.existsLocked(ctx, userID, token)
		return err
	}); err != nil {
		return false, err
	}
	return exists, nil
}

func (r *CacheSessionRepository) ExistsByUser(ctx context.Context, userID int64) (bool, error) {
	var (
		exists bool
		err    error
	)
	if err = r.withUserLock(userID, func() error {
		exists, err = r.existsByUserLocked(ctx, userID)
		return err
	}); err != nil {
		return false, err
	}
	return exists, nil
}

func (r *CacheSessionRepository) WithUserLock(ctx context.Context, userID int64, fn func(port.SessionRepositoryScope) error) error {
	if fn == nil {
		return nil
	}
	return r.withUserLock(userID, func() error {
		return fn(sessionRepositoryScope{repo: r})
	})
}

func (r *CacheSessionRepository) withUserLock(userID int64, fn func() error) error {
	if fn == nil {
		return nil
	}
	lock := r.userLock(userID)
	lock.Lock()
	defer lock.Unlock()
	return fn()
}

func (r *CacheSessionRepository) userLock(userID int64) *sync.Mutex {
	actual, _ := r.userLocks.LoadOrStore(userSessionLockKey(userID), &sync.Mutex{})
	return actual.(*sync.Mutex)
}

func (r *CacheSessionRepository) existsLocked(ctx context.Context, userID int64, token string) (bool, error) {
	_, exists, err := r.cache.Get(ctx, sessionCacheKey(userID, token))
	return exists, err
}

func (r *CacheSessionRepository) existsByUserLocked(ctx context.Context, userID int64) (bool, error) {
	return r.cache.ExistsByPrefix(ctx, sessionCachePrefix(userID))
}

func sessionCachePrefix(userID int64) string {
	return fmt.Sprintf("session:user:%d:", userID)
}

func sessionCacheKey(userID int64, token string) string {
	return sessionCachePrefix(userID) + token
}

func userSessionLockKey(userID int64) int64 {
	return userID
}

func (s sessionRepositoryScope) Exists(ctx context.Context, userID int64, token string) (bool, error) {
	return s.repo.existsLocked(ctx, userID, token)
}

func (s sessionRepositoryScope) ExistsByUser(ctx context.Context, userID int64) (bool, error) {
	return s.repo.existsByUserLocked(ctx, userID)
}

func (s sessionRepositoryScope) Save(ctx context.Context, userID int64, token string, ttlSeconds int) error {
	return s.repo.cache.SetWithTTL(ctx, sessionCacheKey(userID, token), userID, ttlSeconds)
}

func (s sessionRepositoryScope) Delete(ctx context.Context, userID int64, token string) error {
	return s.repo.cache.Delete(ctx, sessionCacheKey(userID, token))
}

func (s sessionRepositoryScope) DeleteByUser(ctx context.Context, userID int64) error {
	_, err := s.repo.cache.DeleteByPrefix(ctx, sessionCachePrefix(userID))
	return err
}
