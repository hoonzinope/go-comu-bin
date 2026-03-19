package service

import (
	"context"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
)

var _ port.GuestCleanupUseCase = (*GuestCleanupService)(nil)

type GuestCleanupService struct {
	userRepository     port.UserRepository
	postRepository     port.PostRepository
	commentRepository  port.CommentRepository
	reactionRepository port.ReactionRepository
	reportRepository   port.ReportRepository
	sessionRepository  port.SessionRepository
	unitOfWork         port.UnitOfWork
}

func NewGuestCleanupService(userRepository port.UserRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, reportRepository port.ReportRepository, sessionRepository port.SessionRepository, unitOfWork port.UnitOfWork) *GuestCleanupService {
	return &GuestCleanupService{
		userRepository:     userRepository,
		postRepository:     postRepository,
		commentRepository:  commentRepository,
		reactionRepository: reactionRepository,
		reportRepository:   reportRepository,
		sessionRepository:  sessionRepository,
		unitOfWork:         unitOfWork,
	}
}

func (s *GuestCleanupService) CleanupGuests(ctx context.Context, now time.Time, pendingGrace, activeUnusedGrace time.Duration, limit int) (int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if limit <= 0 || pendingGrace <= 0 || activeUnusedGrace <= 0 {
		return 0, nil
	}
	items, err := s.userRepository.SelectGuestCleanupCandidates(ctx, now, pendingGrace, activeUnusedGrace, limit)
	if err != nil {
		return 0, customerror.WrapRepository("select guest cleanup candidates", err)
	}
	deletedCount := 0
	for _, item := range items {
		select {
		case <-ctx.Done():
			return deletedCount, ctx.Err()
		default:
		}
		deleted, err := s.cleanupGuest(ctx, item.ID)
		if err != nil {
			return deletedCount, err
		}
		if deleted {
			deletedCount++
		}
	}
	return deletedCount, nil
}

func (s *GuestCleanupService) cleanupGuest(ctx context.Context, userID int64) (bool, error) {
	deleted := false
	err := s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		user, err := tx.UserRepository().SelectUserByID(txCtx, userID)
		if err != nil {
			return customerror.WrapRepository("select user by id for guest cleanup", err)
		}
		if user == nil || !user.IsGuest() {
			return nil
		}
		hasSessions, err := s.sessionRepository.ExistsByUser(txCtx, userID)
		if err != nil {
			return customerror.WrapRepository("check guest sessions for cleanup", err)
		}
		if hasSessions {
			return nil
		}
		hasPosts, err := tx.PostRepository().ExistsByAuthorIDIncludingDeleted(txCtx, userID)
		if err != nil {
			return customerror.WrapRepository("check guest posts for cleanup", err)
		}
		if hasPosts {
			return nil
		}
		hasComments, err := tx.CommentRepository().ExistsByAuthorIDIncludingDeleted(txCtx, userID)
		if err != nil {
			return customerror.WrapRepository("check guest comments for cleanup", err)
		}
		if hasComments {
			return nil
		}
		hasReactions, err := tx.ReactionRepository().ExistsByUserID(txCtx, userID)
		if err != nil {
			return customerror.WrapRepository("check guest reactions for cleanup", err)
		}
		if hasReactions {
			return nil
		}
		hasReports, err := tx.ReportRepository().ExistsByReporterUserID(txCtx, userID)
		if err != nil {
			return customerror.WrapRepository("check guest reports for cleanup", err)
		}
		if hasReports {
			return nil
		}
		user.SoftDelete()
		if err := tx.UserRepository().Update(txCtx, user); err != nil {
			return customerror.WrapRepository("soft delete guest for cleanup", err)
		}
		deleted = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return deleted, nil
}
