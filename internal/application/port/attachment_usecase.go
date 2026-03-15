package port

import (
	"context"
	"io"

	"github.com/hoonzinope/go-comu-bin/internal/application/model"
)

type AttachmentUseCase interface {
	CreatePostAttachment(ctx context.Context, postUUID string, userID int64, fileName, contentType string, sizeBytes int64, storageKey string) (string, error)
	UploadPostAttachment(ctx context.Context, postUUID string, userID int64, fileName, contentType string, content io.Reader) (*model.AttachmentUpload, error)
	GetPostAttachments(ctx context.Context, postUUID string) ([]model.Attachment, error)
	GetPostAttachmentFile(ctx context.Context, postUUID, attachmentUUID string) (*model.AttachmentFile, error)
	GetPostAttachmentPreviewFile(ctx context.Context, postUUID, attachmentUUID string, userID int64) (*model.AttachmentFile, error)
	DeletePostAttachment(ctx context.Context, postUUID, attachmentUUID string, userID int64) error
}
