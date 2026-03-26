package attachment

import (
	"context"
	svccommon "github.com/hoonzinope/go-comu-bin/internal/application/service/common"
	"io"
	"log/slog"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
)

var _ port.AttachmentUseCase = (*AttachmentService)(nil)
var _ port.AttachmentCleanupUseCase = (*AttachmentService)(nil)

type AttachmentService struct {
	queryHandler    *attachmentQueryHandler
	commandHandler  *attachmentCommandHandler
	cleanupWorkflow *attachmentCleanupWorkflow
}

func NewAttachmentService(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, attachmentRepository port.AttachmentRepository, unitOfWork port.UnitOfWork, fileStorage port.FileStorage, cache port.Cache, maxUploadSizeBytes int64, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *AttachmentService {
	return NewAttachmentServiceWithActionDispatcher(userRepository, boardRepository, postRepository, attachmentRepository, unitOfWork, fileStorage, cache, nil, maxUploadSizeBytes, ImageOptimizationConfig{Enabled: true, JPEGQuality: 82}, authorizationPolicy, logger...)
}

func NewAttachmentServiceWithOptions(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, attachmentRepository port.AttachmentRepository, unitOfWork port.UnitOfWork, fileStorage port.FileStorage, cache port.Cache, maxUploadSizeBytes int64, imageOptimization ImageOptimizationConfig, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *AttachmentService {
	return NewAttachmentServiceWithActionDispatcher(userRepository, boardRepository, postRepository, attachmentRepository, unitOfWork, fileStorage, cache, nil, maxUploadSizeBytes, imageOptimization, authorizationPolicy, logger...)
}

func NewAttachmentServiceWithActionDispatcher(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, attachmentRepository port.AttachmentRepository, unitOfWork port.UnitOfWork, fileStorage port.FileStorage, cache port.Cache, actionDispatcher port.ActionHookDispatcher, maxUploadSizeBytes int64, imageOptimization ImageOptimizationConfig, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *AttachmentService {
	resolvedLogger := svccommon.ResolveLogger(logger)
	return &AttachmentService{
		queryHandler:    newAttachmentQueryHandler(userRepository, boardRepository, postRepository, attachmentRepository, fileStorage, authorizationPolicy),
		commandHandler:  newAttachmentCommandHandler(boardRepository, postRepository, userRepository, unitOfWork, fileStorage, svccommon.ResolveActionDispatcher(actionDispatcher), maxUploadSizeBytes, imageOptimization, authorizationPolicy, resolvedLogger),
		cleanupWorkflow: newAttachmentCleanupWorkflow(attachmentRepository, fileStorage),
	}
}

func (s *AttachmentService) CreatePostAttachment(ctx context.Context, postUUID string, userID int64, fileName, contentType string, sizeBytes int64, storageKey string) (string, error) {
	return s.commandHandler.CreatePostAttachment(ctx, postUUID, userID, fileName, contentType, sizeBytes, storageKey)
}

func (s *AttachmentService) UploadPostAttachment(ctx context.Context, postUUID string, userID int64, fileName, contentType string, content io.Reader) (*model.AttachmentUpload, error) {
	return s.commandHandler.UploadPostAttachment(ctx, postUUID, userID, fileName, contentType, content)
}

func (s *AttachmentService) GetPostAttachments(ctx context.Context, postUUID string) ([]model.Attachment, error) {
	return s.queryHandler.GetPostAttachments(ctx, postUUID)
}

func (s *AttachmentService) GetPostAttachmentFile(ctx context.Context, postUUID, attachmentUUID string) (*model.AttachmentFile, error) {
	return s.queryHandler.GetPostAttachmentFile(ctx, postUUID, attachmentUUID)
}

func (s *AttachmentService) GetPostAttachmentPreviewFile(ctx context.Context, postUUID, attachmentUUID string, userID int64) (*model.AttachmentFile, error) {
	return s.queryHandler.GetPostAttachmentPreviewFile(ctx, postUUID, attachmentUUID, userID)
}

func (s *AttachmentService) DeletePostAttachment(ctx context.Context, postUUID, attachmentUUID string, userID int64) error {
	return s.commandHandler.DeletePostAttachment(ctx, postUUID, attachmentUUID, userID)
}

func (s *AttachmentService) CleanupAttachments(ctx context.Context, now time.Time, gracePeriod time.Duration, limit int) (int, error) {
	return s.cleanupWorkflow.CleanupAttachments(ctx, now, gracePeriod, limit)
}
