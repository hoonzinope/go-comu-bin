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
	OrphanedAt  *time.Time
}

func NewAttachment(postID int64, fileName, contentType string, sizeBytes int64, storageKey string) *Attachment {
	now := time.Now()
	return &Attachment{
		PostID:      postID,
		FileName:    fileName,
		ContentType: contentType,
		SizeBytes:   sizeBytes,
		StorageKey:  storageKey,
		CreatedAt:   now,
		OrphanedAt:  &now,
	}
}

func (a *Attachment) MarkReferenced() {
	a.OrphanedAt = nil
}

func (a *Attachment) MarkOrphaned() {
	if a.OrphanedAt != nil {
		return
	}
	now := time.Now()
	a.OrphanedAt = &now
}

func (a *Attachment) IsOrphaned() bool {
	return a.OrphanedAt != nil
}
