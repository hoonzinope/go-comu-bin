package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUser_NewUserAndIsAdmin(t *testing.T) {
	u := NewUser("alice", "pw")
	assert.Equal(t, "alice", u.Name)
	assert.Equal(t, "pw", u.Password)
	assert.Equal(t, "user", u.Role)
	assert.False(t, u.IsAdmin())
	assert.False(t, u.CreatedAt.IsZero())
}

func TestUser_NewAdminAndIsAdmin(t *testing.T) {
	u := NewAdmin("admin", "pw")
	assert.Equal(t, "admin", u.Role)
	assert.True(t, u.IsAdmin())
	assert.False(t, u.CreatedAt.IsZero())
}
