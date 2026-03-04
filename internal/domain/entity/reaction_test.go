package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReaction_NewReactionAndUpdateReaction(t *testing.T) {
	r := NewReaction("post", 3, "like", 7)
	assert.Equal(t, "post", r.TargetType)
	assert.EqualValues(t, 3, r.TargetID)
	assert.Equal(t, "like", r.Type)
	assert.EqualValues(t, 7, r.UserID)
	assert.False(t, r.CreatedAt.IsZero())

	r.Update("dislike")
	assert.Equal(t, "dislike", r.Type)
}
