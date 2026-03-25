package emailverificationcleanup

import (
	"context"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
)

var _ port.EmailVerificationCleanupUseCase = (*Service)(nil)

type Service struct {
	verificationTokens port.EmailVerificationTokenRepository
}

func NewService(verificationTokens port.EmailVerificationTokenRepository) *Service {
	return &Service{verificationTokens: verificationTokens}
}

func (s *Service) CleanupEmailVerificationTokens(ctx context.Context, now time.Time, gracePeriod time.Duration, limit int) (int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if limit <= 0 || gracePeriod <= 0 {
		return 0, nil
	}
	deleted, err := s.verificationTokens.DeleteExpiredOrConsumedBefore(ctx, now.Add(-gracePeriod), limit)
	if err != nil {
		return 0, customerror.WrapRepository("cleanup email verification tokens", err)
	}
	return deleted, nil
}
