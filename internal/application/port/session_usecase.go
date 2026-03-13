package port

import "context"

type SessionUseCase interface {
	Login(ctx context.Context, username, password string) (string, error)
	Logout(ctx context.Context, token string) error
	InvalidateUserSessions(ctx context.Context, userID int64) error
	ValidateTokenToId(ctx context.Context, token string) (int64, error)
}
