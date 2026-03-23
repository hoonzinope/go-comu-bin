package port

import (
	"context"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type UserRepository interface {
	Save(ctx context.Context, user *entity.User) (int64, error)
	SelectUserByUsername(ctx context.Context, username string) (*entity.User, error)
	SelectUserByEmail(ctx context.Context, email string) (*entity.User, error)
	SelectUserByUUID(ctx context.Context, userUUID string) (*entity.User, error)
	SelectUserByID(ctx context.Context, id int64) (*entity.User, error)
	SelectUserByIDIncludingDeleted(ctx context.Context, id int64) (*entity.User, error)
	SelectUsersByIDsIncludingDeleted(ctx context.Context, ids []int64) (map[int64]*entity.User, error)
	SelectGuestCleanupCandidates(ctx context.Context, now time.Time, pendingGrace, activeUnusedGrace time.Duration, limit int) ([]*entity.User, error)
	Update(ctx context.Context, user *entity.User) error
	Delete(ctx context.Context, id int64) error
}
