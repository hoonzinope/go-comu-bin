package port

import "context"

type AccountUseCase interface {
	DeleteMyAccount(ctx context.Context, userID int64, password string) error
	UpgradeGuestAccount(ctx context.Context, userID int64, currentToken, username, email, password string) (string, error)
	RequestEmailVerification(ctx context.Context, userID int64) error
	ConfirmEmailVerification(ctx context.Context, token string) error
	RequestPasswordReset(ctx context.Context, email string) error
	ConfirmPasswordReset(ctx context.Context, token, newPassword string) error
}
