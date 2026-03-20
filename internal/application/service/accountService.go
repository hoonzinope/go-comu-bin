package service

import (
	"log/slog"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	accountsvc "github.com/hoonzinope/go-comu-bin/internal/application/service/account"
)

type AccountService = accountsvc.Service

func NewAccountService(userUseCase port.UserUseCase, sessionUseCase port.SessionUseCase, logger ...*slog.Logger) *AccountService {
	return accountsvc.NewService(userUseCase, sessionUseCase, logger...)
}
