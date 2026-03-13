package port

import "context"

type AccountUseCase interface {
	DeleteMyAccount(ctx context.Context, userID int64, password string) error
}
