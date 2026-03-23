package auth

import "github.com/hoonzinope/go-comu-bin/internal/application/port"

var _ port.EmailVerificationTokenIssuer = (*EmailVerificationTokenIssuer)(nil)

type EmailVerificationTokenIssuer struct {
	issuer *PasswordResetTokenIssuer
}

func NewEmailVerificationTokenIssuer() *EmailVerificationTokenIssuer {
	return &EmailVerificationTokenIssuer{issuer: NewPasswordResetTokenIssuer()}
}

func (i *EmailVerificationTokenIssuer) Issue() (string, error) {
	return i.issuer.Issue()
}
