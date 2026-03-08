package entity

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPost_NewPostAndUpdatePost(t *testing.T) {
	p := NewPost("title", "content", 10, 20)
	assert.Equal(t, "title", p.Title)
	assert.Equal(t, "content", p.Content)
	assert.EqualValues(t, 10, p.AuthorID)
	assert.EqualValues(t, 20, p.BoardID)
	assert.Equal(t, PostStatusPublished, p.Status)
	assert.Nil(t, p.DeletedAt)
	assert.False(t, p.CreatedAt.IsZero())
	assert.False(t, p.UpdatedAt.IsZero())

	before := p.UpdatedAt
	time.Sleep(time.Millisecond)
	p.Update("new-title", "new-content")

	assert.Equal(t, "new-title", p.Title)
	assert.Equal(t, "new-content", p.Content)
	assert.True(t, p.UpdatedAt.After(before))
}

func TestPost_SoftDelete(t *testing.T) {
	p := NewPost("title", "content", 10, 20)

	p.SoftDelete()

	assert.Equal(t, PostStatusDeleted, p.Status)
	assert.NotNil(t, p.DeletedAt)
}

func TestPost_NewDraftPost(t *testing.T) {
	p := NewDraftPost("title", "content", 10, 20)

	assert.Equal(t, PostStatusDraft, p.Status)
	assert.Nil(t, p.DeletedAt)
}

func TestPost_Publish(t *testing.T) {
	p := NewDraftPost("title", "content", 10, 20)

	p.Publish()

	assert.Equal(t, PostStatusPublished, p.Status)
	assert.Nil(t, p.DeletedAt)
}
