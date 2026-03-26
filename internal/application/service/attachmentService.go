package service

import (
	"log/slog"

	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	attachmentsvc "github.com/hoonzinope/go-comu-bin/internal/application/service/attachment"
)

type ImageOptimizationConfig = attachmentsvc.ImageOptimizationConfig

func NewAttachmentService(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, attachmentRepository port.AttachmentRepository, unitOfWork port.UnitOfWork, fileStorage port.FileStorage, cache port.Cache, maxUploadSizeBytes int64, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *attachmentsvc.AttachmentService {
	return attachmentsvc.NewAttachmentService(userRepository, boardRepository, postRepository, attachmentRepository, unitOfWork, fileStorage, cache, maxUploadSizeBytes, authorizationPolicy, logger...)
}

func NewAttachmentServiceWithOptions(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, attachmentRepository port.AttachmentRepository, unitOfWork port.UnitOfWork, fileStorage port.FileStorage, cache port.Cache, maxUploadSizeBytes int64, imageOptimization ImageOptimizationConfig, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *attachmentsvc.AttachmentService {
	return attachmentsvc.NewAttachmentServiceWithOptions(userRepository, boardRepository, postRepository, attachmentRepository, unitOfWork, fileStorage, cache, maxUploadSizeBytes, imageOptimization, authorizationPolicy, logger...)
}

func NewAttachmentServiceWithActionDispatcher(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, attachmentRepository port.AttachmentRepository, unitOfWork port.UnitOfWork, fileStorage port.FileStorage, cache port.Cache, actionDispatcher port.ActionHookDispatcher, maxUploadSizeBytes int64, imageOptimization ImageOptimizationConfig, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *attachmentsvc.AttachmentService {
	return attachmentsvc.NewAttachmentServiceWithActionDispatcher(userRepository, boardRepository, postRepository, attachmentRepository, unitOfWork, fileStorage, cache, actionDispatcher, maxUploadSizeBytes, imageOptimization, authorizationPolicy, logger...)
}
