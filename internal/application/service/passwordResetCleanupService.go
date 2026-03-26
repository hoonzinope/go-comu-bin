package service

import (
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	passwordresetcleanupsvc "github.com/hoonzinope/go-comu-bin/internal/application/service/passwordresetcleanup"
)

func NewPasswordResetCleanupService(resetTokens port.PasswordResetTokenRepository) *passwordresetcleanupsvc.Service {
	return passwordresetcleanupsvc.NewService(resetTokens)
}
