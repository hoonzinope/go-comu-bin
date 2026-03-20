package port

import "context"

type SessionRepositoryScope interface {
	Exists(ctx context.Context, userID int64, token string) (bool, error)
	ExistsByUser(ctx context.Context, userID int64) (bool, error)
}

type SessionRepository interface {
	Save(ctx context.Context, userID int64, token string, ttlSeconds int) error
	Delete(ctx context.Context, userID int64, token string) error
	DeleteByUser(ctx context.Context, userID int64) error
	Exists(ctx context.Context, userID int64, token string) (bool, error)
	ExistsByUser(ctx context.Context, userID int64) (bool, error)
	WithUserLock(ctx context.Context, userID int64, fn func(SessionRepositoryScope) error) error
}
