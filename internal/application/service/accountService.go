package service

import (
	"log/slog"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
)

var _ port.AccountUseCase = (*AccountService)(nil)

type AccountService struct {
	userUseCase    port.UserUseCase
	sessionUseCase port.SessionUseCase
}

func NewAccountService(userUseCase port.UserUseCase, sessionUseCase port.SessionUseCase) *AccountService {
	return &AccountService{
		userUseCase:    userUseCase,
		sessionUseCase: sessionUseCase,
	}
}

func (s *AccountService) DeleteMyAccount(userID int64, password string) error {
	if err := s.userUseCase.DeleteMe(userID, password); err != nil {
		return err
	}
	if err := s.sessionUseCase.InvalidateUserSessions(userID); err != nil {
		slog.Warn("failed to invalidate deleted user sessions", "user_id", userID, "error", err)
	}
	return nil
}
