package service

import (
	"log/slog"

	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	attachmentsvc "github.com/hoonzinope/go-comu-bin/internal/application/service/attachment"
)

type AttachmentService = attachmentsvc.Service
type ImageOptimizationConfig = attachmentsvc.ImageOptimizationConfig

func NewAttachmentService(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, attachmentRepository port.AttachmentRepository, unitOfWork port.UnitOfWork, fileStorage port.FileStorage, cache port.Cache, maxUploadSizeBytes int64, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *AttachmentService {
	return attachmentsvc.NewService(userRepository, boardRepository, postRepository, attachmentRepository, unitOfWork, fileStorage, cache, maxUploadSizeBytes, authorizationPolicy, logger...)
}

func NewAttachmentServiceWithOptions(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, attachmentRepository port.AttachmentRepository, unitOfWork port.UnitOfWork, fileStorage port.FileStorage, cache port.Cache, maxUploadSizeBytes int64, imageOptimization ImageOptimizationConfig, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *AttachmentService {
	return attachmentsvc.NewServiceWithOptions(userRepository, boardRepository, postRepository, attachmentRepository, unitOfWork, fileStorage, cache, maxUploadSizeBytes, imageOptimization, authorizationPolicy, logger...)
}

func NewAttachmentServiceWithActionDispatcher(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, attachmentRepository port.AttachmentRepository, unitOfWork port.UnitOfWork, fileStorage port.FileStorage, cache port.Cache, actionDispatcher port.ActionHookDispatcher, maxUploadSizeBytes int64, imageOptimization ImageOptimizationConfig, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *AttachmentService {
	return attachmentsvc.NewServiceWithActionDispatcher(userRepository, boardRepository, postRepository, attachmentRepository, unitOfWork, fileStorage, cache, actionDispatcher, maxUploadSizeBytes, imageOptimization, authorizationPolicy, logger...)
}
