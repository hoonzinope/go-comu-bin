package outboxadmin

import (
	"context"
	svccommon "github.com/hoonzinope/go-comu-bin/internal/application/service/common"
	"log/slog"
	"strings"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
)

var _ port.OutboxAdminUseCase = (*OutboxAdminService)(nil)

type Service = OutboxAdminService

type OutboxAdminService struct {
	userRepository      port.UserRepository
	outboxStore         port.OutboxStore
	authorizationPolicy policy.AuthorizationPolicy
	logger              *slog.Logger
}

func NewOutboxAdminService(userRepository port.UserRepository, outboxStore port.OutboxStore, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *OutboxAdminService {
	return &OutboxAdminService{
		userRepository:      userRepository,
		outboxStore:         outboxStore,
		authorizationPolicy: authorizationPolicy,
		logger:              svccommon.ResolveLogger(logger),
	}
}

func NewService(userRepository port.UserRepository, outboxStore port.OutboxStore, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *Service {
	return NewOutboxAdminService(userRepository, outboxStore, authorizationPolicy, logger...)
}

func (s *OutboxAdminService) GetDeadMessages(ctx context.Context, adminID int64, limit int, lastID string) (*model.OutboxDeadMessageList, error) {
	if err := svccommon.RequirePositiveLimit(limit); err != nil {
		return nil, err
	}
	if err := s.ensureAdmin(ctx, adminID); err != nil {
		return nil, err
	}
	lastID = strings.TrimSpace(lastID)
	messages, err := s.outboxStore.SelectDead(limit+1, lastID)
	if err != nil {
		return nil, customerror.WrapRepository("select dead outbox list", err)
	}
	hasMore := false
	var nextLastID *string
	if len(messages) > limit {
		hasMore = true
		messages = messages[:limit]
	}
	if hasMore && len(messages) > 0 {
		next := messages[len(messages)-1].ID
		nextLastID = &next
	}
	views := make([]model.OutboxDeadMessage, 0, len(messages))
	for _, message := range messages {
		views = append(views, model.OutboxDeadMessage{
			ID:            message.ID,
			EventName:     message.EventName,
			AttemptCount:  message.AttemptCount,
			LastError:     message.LastError,
			OccurredAt:    message.OccurredAt,
			NextAttemptAt: message.NextAttemptAt,
		})
	}
	return &model.OutboxDeadMessageList{
		Messages:   views,
		Limit:      limit,
		LastID:     lastID,
		HasMore:    hasMore,
		NextLastID: nextLastID,
	}, nil
}

func (s *OutboxAdminService) RequeueDeadMessage(ctx context.Context, adminID int64, messageID string) error {
	if err := s.ensureAdmin(ctx, adminID); err != nil {
		return err
	}
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return customerror.ErrInvalidInput
	}
	if err := s.ensureDeadMessage(messageID); err != nil {
		return err
	}
	if err := s.outboxStore.MarkRetry(messageID, time.Now(), "manual requeue by admin"); err != nil {
		return customerror.WrapRepository("requeue dead outbox message", err)
	}
	s.logger.Info("admin requeued dead outbox message", "message_id", messageID, "admin_id", adminID)
	return nil
}

func (s *OutboxAdminService) DiscardDeadMessage(ctx context.Context, adminID int64, messageID string) error {
	if err := s.ensureAdmin(ctx, adminID); err != nil {
		return err
	}
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return customerror.ErrInvalidInput
	}
	if err := s.ensureDeadMessage(messageID); err != nil {
		return err
	}
	if err := s.outboxStore.MarkSucceeded(messageID); err != nil {
		return customerror.WrapRepository("discard dead outbox message", err)
	}
	s.logger.Info("admin discarded dead outbox message", "message_id", messageID, "admin_id", adminID)
	return nil
}

func (s *OutboxAdminService) ensureAdmin(ctx context.Context, adminID int64) error {
	_, err := svccommon.RequireAdminUser(ctx, s.userRepository, s.authorizationPolicy, adminID, "outbox admin")
	return err
}

func (s *OutboxAdminService) ensureDeadMessage(messageID string) error {
	message, err := s.outboxStore.SelectByID(messageID)
	if err != nil {
		return customerror.WrapRepository("select outbox message by id for dead operation", err)
	}
	if message == nil || message.Status != port.OutboxStatusDead {
		return customerror.ErrInvalidInput
	}
	return nil
}
