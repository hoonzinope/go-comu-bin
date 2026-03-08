package port

import (
	"io"

	"github.com/hoonzinope/go-comu-bin/internal/application/model"
)

type AttachmentUseCase interface {
	CreatePostAttachment(postID, userID int64, fileName, contentType string, sizeBytes int64, storageKey string) (int64, error)
	UploadPostAttachment(postID, userID int64, fileName, contentType string, content io.Reader) (int64, error)
	GetPostAttachments(postID int64) ([]model.Attachment, error)
	DeletePostAttachment(postID, attachmentID, userID int64) error
}
