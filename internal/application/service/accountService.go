package service

import (
	"log/slog"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	accountsvc "github.com/hoonzinope/go-comu-bin/internal/application/service/account"
)

func NewAccountService(userUseCase port.UserUseCase, sessionUseCase port.SessionUseCase, logger ...*slog.Logger) *accountsvc.AccountService {
	return accountsvc.NewAccountService(userUseCase, sessionUseCase, logger...)
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
	args ...any,
) *accountsvc.AccountService {
	var (
		verificationTokenTTL time.Duration
		resetTokens          port.PasswordResetTokenRepository
		resetIssuer          port.PasswordResetTokenIssuer
		resetTokenTTL        time.Duration
		loggers              []*slog.Logger
	)
	for _, arg := range args {
		switch v := arg.(type) {
		case port.EmailVerificationMailSender:
			// ignored; mail delivery moved to outbox relay
		case port.PasswordResetMailSender:
			// ignored; mail delivery moved to outbox relay
		case time.Duration:
			if verificationTokenTTL == 0 {
				verificationTokenTTL = v
			} else {
				resetTokenTTL = v
			}
		case port.PasswordResetTokenRepository:
			resetTokens = v
		case port.PasswordResetTokenIssuer:
			resetIssuer = v
		case *slog.Logger:
			loggers = append(loggers, v)
		}
	}
	return accountsvc.NewAccountServiceWithGuestUpgrade(
		userUseCase,
		sessionUseCase,
		userRepository,
		unitOfWork,
		passwordHasher,
		tokenProvider,
		sessionRepository,
		verificationTokens,
		verificationIssuer,
		verificationTokenTTL,
		resetTokens,
		resetIssuer,
		resetTokenTTL,
		loggers...,
	)
}
