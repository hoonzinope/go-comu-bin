package service

import (
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	emailverificationcleanupsvc "github.com/hoonzinope/go-comu-bin/internal/application/service/emailverificationcleanup"
)

type EmailVerificationCleanupService = emailverificationcleanupsvc.Service

func NewEmailVerificationCleanupService(verificationTokens port.EmailVerificationTokenRepository) *EmailVerificationCleanupService {
	return emailverificationcleanupsvc.NewService(verificationTokens)
}
