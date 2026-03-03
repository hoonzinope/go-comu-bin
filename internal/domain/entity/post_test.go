package entity

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPost_NewPostAndUpdatePost(t *testing.T) {
	p := &Post{}

	p.NewPost("title", "content", 10, 20)
	assert.Equal(t, "title", p.Title)
	assert.Equal(t, "content", p.Content)
	assert.EqualValues(t, 10, p.AuthorID)
	assert.EqualValues(t, 20, p.BoardID)
	assert.False(t, p.CreatedAt.IsZero())
	assert.False(t, p.UpdatedAt.IsZero())

	before := p.UpdatedAt
	time.Sleep(time.Millisecond)
	p.UpdatePost("new-title", "new-content")

	assert.Equal(t, "new-title", p.Title)
	assert.Equal(t, "new-content", p.Content)
	assert.True(t, p.UpdatedAt.After(before))
}
