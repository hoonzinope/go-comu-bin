package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComment_NewCommentAndUpdateComment(t *testing.T) {
	parentID := int64(9)
	c := &Comment{}

	c.NewComment("hello", 1, 2, &parentID)
	assert.Equal(t, "hello", c.Content)
	assert.EqualValues(t, 1, c.AuthorID)
	assert.EqualValues(t, 2, c.PostID)
	require.NotNil(t, c.ParentID)
	assert.EqualValues(t, parentID, *c.ParentID)
	assert.False(t, c.CreatedAt.IsZero())

	c.UpdateComment("updated")
	assert.Equal(t, "updated", c.Content)
}
