package passwordresetcleanup

import (
	"context"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
)

var _ port.PasswordResetCleanupUseCase = (*Service)(nil)

type Service struct {
	resetTokens port.PasswordResetTokenRepository
}

func NewService(resetTokens port.PasswordResetTokenRepository) *Service {
	return &Service{resetTokens: resetTokens}
}

func (s *Service) CleanupPasswordResetTokens(ctx context.Context, now time.Time, gracePeriod time.Duration, limit int) (int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if limit <= 0 || gracePeriod <= 0 {
		return 0, nil
	}
	deleted, err := s.resetTokens.DeleteExpiredOrConsumedBefore(ctx, now.Add(-gracePeriod), limit)
	if err != nil {
		return 0, customerror.WrapRepository("cleanup password reset tokens", err)
	}
	return deleted, nil
}
