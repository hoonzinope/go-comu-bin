package model

import "io"

type AttachmentFile struct {
	FileName    string
	ContentType string
	SizeBytes   int64
	ETag        string
	Content     io.ReadCloser
}
