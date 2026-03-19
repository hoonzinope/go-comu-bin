package port

import "context"

type GuestAccountIssuer interface {
	IssueGuestAccount(ctx context.Context) (int64, error)
}
