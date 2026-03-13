package port

import (
	"context"
	"io"

	"github.com/hoonzinope/go-comu-bin/internal/application/model"
)

type AttachmentUseCase interface {
	CreatePostAttachment(ctx context.Context, postID, userID int64, fileName, contentType string, sizeBytes int64, storageKey string) (int64, error)
	UploadPostAttachment(ctx context.Context, postID, userID int64, fileName, contentType string, content io.Reader) (*model.AttachmentUpload, error)
	GetPostAttachments(ctx context.Context, postID int64) ([]model.Attachment, error)
	GetPostAttachmentFile(ctx context.Context, postID, attachmentID int64) (*model.AttachmentFile, error)
	GetPostAttachmentPreviewFile(ctx context.Context, postID, attachmentID, userID int64) (*model.AttachmentFile, error)
	DeletePostAttachment(ctx context.Context, postID, attachmentID, userID int64) error
}
