package port

import "context"

type CredentialVerifier interface {
	VerifyCredentials(ctx context.Context, username, password string) (int64, error)
}
