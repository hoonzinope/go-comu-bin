package service

import (
	"context"
	"log/slog"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
)

var _ port.AccountUseCase = (*AccountService)(nil)

type AccountService struct {
	userUseCase    port.UserUseCase
	sessionUseCase port.SessionUseCase
	logger         *slog.Logger
}

func NewAccountService(userUseCase port.UserUseCase, sessionUseCase port.SessionUseCase, logger ...*slog.Logger) *AccountService {
	return &AccountService{
		userUseCase:    userUseCase,
		sessionUseCase: sessionUseCase,
		logger:         resolveLogger(logger),
	}
}

func (s *AccountService) DeleteMyAccount(ctx context.Context, userID int64, password string) error {
	if err := s.userUseCase.DeleteMe(ctx, userID, password); err != nil {
		return err
	}
	if err := s.sessionUseCase.InvalidateUserSessions(ctx, userID); err != nil {
		s.logger.Warn("failed to invalidate deleted user sessions", "user_id", userID, "error", err)
	}
	return nil
}
