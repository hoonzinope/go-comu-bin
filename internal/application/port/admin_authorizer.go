package port

import "context"

type AdminAuthorizer interface {
	EnsureAdmin(ctx context.Context, userID int64) error
}
