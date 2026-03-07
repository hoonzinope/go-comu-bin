package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReaction_NewReactionAndUpdateReaction(t *testing.T) {
	r := NewReaction(ReactionTargetPost, 3, ReactionTypeLike, 7)
	assert.Equal(t, ReactionTargetPost, r.TargetType)
	assert.EqualValues(t, 3, r.TargetID)
	assert.Equal(t, ReactionTypeLike, r.Type)
	assert.EqualValues(t, 7, r.UserID)
	assert.False(t, r.CreatedAt.IsZero())

	r.Update(ReactionTypeDislike)
	assert.Equal(t, ReactionTypeDislike, r.Type)
}
