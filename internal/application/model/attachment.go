package model

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
