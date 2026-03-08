package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAttachment_NewAttachment(t *testing.T) {
	a := NewAttachment(3, "a.png", "image/png", 1024, "attachments/a.png")

	assert.Equal(t, int64(3), a.PostID)
	assert.Equal(t, "a.png", a.FileName)
	assert.Equal(t, "image/png", a.ContentType)
	assert.Equal(t, int64(1024), a.SizeBytes)
	assert.Equal(t, "attachments/a.png", a.StorageKey)
	assert.False(t, a.CreatedAt.IsZero())
}
