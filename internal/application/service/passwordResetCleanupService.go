package service

import (
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	passwordresetcleanupsvc "github.com/hoonzinope/go-comu-bin/internal/application/service/passwordresetcleanup"
)

type PasswordResetCleanupService = passwordresetcleanupsvc.Service

func NewPasswordResetCleanupService(resetTokens port.PasswordResetTokenRepository) *PasswordResetCleanupService {
	return passwordresetcleanupsvc.NewService(resetTokens)
}
