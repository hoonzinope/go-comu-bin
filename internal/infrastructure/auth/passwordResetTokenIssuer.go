package auth

import (
	"crypto/rand"
	"encoding/base64"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
)

var _ port.PasswordResetTokenIssuer = (*PasswordResetTokenIssuer)(nil)

type PasswordResetTokenIssuer struct{}

func NewPasswordResetTokenIssuer() *PasswordResetTokenIssuer {
	return &PasswordResetTokenIssuer{}
}

func (i *PasswordResetTokenIssuer) Issue() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
