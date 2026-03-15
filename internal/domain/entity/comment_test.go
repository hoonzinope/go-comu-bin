package entity

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComment_NewCommentAndUpdateComment(t *testing.T) {
	parentID := int64(9)
	c := NewComment("hello", 1, 2, &parentID)
	assert.NotEmpty(t, c.UUID)
	assert.Equal(t, "hello", c.Content)
	assert.EqualValues(t, 1, c.AuthorID)
	assert.EqualValues(t, 2, c.PostID)
	require.NotNil(t, c.ParentID)
	assert.EqualValues(t, parentID, *c.ParentID)
	assert.Equal(t, CommentStatusActive, c.Status)
	assert.Nil(t, c.DeletedAt)
	assert.False(t, c.CreatedAt.IsZero())
	assert.False(t, c.UpdatedAt.IsZero())

	before := c.UpdatedAt
	time.Sleep(time.Millisecond)
	c.Update("updated")
	assert.Equal(t, "updated", c.Content)
	assert.True(t, c.UpdatedAt.After(before))
}

func TestComment_SoftDelete(t *testing.T) {
	c := NewComment("hello", 1, 2, nil)

	c.SoftDelete()

	assert.Equal(t, CommentStatusDeleted, c.Status)
	assert.NotNil(t, c.DeletedAt)
}
