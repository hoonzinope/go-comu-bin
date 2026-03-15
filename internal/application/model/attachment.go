package model

import "time"

type Attachment struct {
	UUID        string
	PostUUID    string
	FileName    string
	ContentType string
	SizeBytes   int64
	StorageKey  string
	PreviewURL  string
	CreatedAt   time.Time
}
