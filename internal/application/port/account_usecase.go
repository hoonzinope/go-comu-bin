package port

import "context"

type AccountUseCase interface {
	DeleteMyAccount(ctx context.Context, userID int64, password string) error
	UpgradeGuestAccount(ctx context.Context, userID int64, currentToken, username, email, password string) (string, error)
}
