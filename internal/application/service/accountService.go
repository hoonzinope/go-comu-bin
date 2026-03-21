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

func NewAccountServiceWithGuestUpgrade(
	userUseCase port.UserUseCase,
	sessionUseCase port.SessionUseCase,
	userRepository port.UserRepository,
	unitOfWork port.UnitOfWork,
	passwordHasher port.PasswordHasher,
	tokenProvider port.TokenProvider,
	sessionRepository port.SessionRepository,
	logger ...*slog.Logger,
) *AccountService {
	return accountsvc.NewServiceWithGuestUpgrade(
		userUseCase,
		sessionUseCase,
		userRepository,
		unitOfWork,
		passwordHasher,
		tokenProvider,
		sessionRepository,
		logger...,
	)
}
