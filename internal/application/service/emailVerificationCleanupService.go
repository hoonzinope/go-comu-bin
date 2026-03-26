package service

import (
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	emailverificationcleanupsvc "github.com/hoonzinope/go-comu-bin/internal/application/service/emailverificationcleanup"
)

func NewEmailVerificationCleanupService(verificationTokens port.EmailVerificationTokenRepository) *emailverificationcleanupsvc.Service {
	return emailverificationcleanupsvc.NewService(verificationTokens)
}
