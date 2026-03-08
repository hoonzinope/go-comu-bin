package entity

import "time"

type Attachment struct {
	ID          int64
	PostID      int64
	FileName    string
	ContentType string
	SizeBytes   int64
	StorageKey  string
	CreatedAt   time.Time
}

func NewAttachment(postID int64, fileName, contentType string, sizeBytes int64, storageKey string) *Attachment {
	return &Attachment{
		PostID:      postID,
		FileName:    fileName,
		ContentType: contentType,
		SizeBytes:   sizeBytes,
		StorageKey:  storageKey,
		CreatedAt:   time.Now(),
	}
}
