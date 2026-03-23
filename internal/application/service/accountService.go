package service

import (
	"log/slog"
	"time"

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
	verificationTokens port.EmailVerificationTokenRepository,
	verificationIssuer port.EmailVerificationTokenIssuer,
	verificationMailer port.EmailVerificationMailSender,
	verificationTokenTTL time.Duration,
	resetTokens port.PasswordResetTokenRepository,
	resetIssuer port.PasswordResetTokenIssuer,
	resetMailer port.PasswordResetMailSender,
	resetTokenTTL time.Duration,
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
		verificationTokens,
		verificationIssuer,
		verificationMailer,
		verificationTokenTTL,
		resetTokens,
		resetIssuer,
		resetMailer,
		resetTokenTTL,
		logger...,
	)
}
