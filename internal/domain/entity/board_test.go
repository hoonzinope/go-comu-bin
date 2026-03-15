package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBoard_NewBoardAndUpdateBoard(t *testing.T) {
	b := NewBoard("free", "desc")
	assert.NotEmpty(t, b.UUID)
	assert.Equal(t, "free", b.Name)
	assert.Equal(t, "desc", b.Description)
	assert.False(t, b.CreatedAt.IsZero())

	b.Update("notice", "updated")
	assert.Equal(t, "notice", b.Name)
	assert.Equal(t, "updated", b.Description)
}
