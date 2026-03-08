package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUser_NewUserAndIsAdmin(t *testing.T) {
	u := NewUser("alice", "pw")
	assert.NotEmpty(t, u.UUID)
	assert.Equal(t, "alice", u.Name)
	assert.Equal(t, "", u.Email)
	assert.Equal(t, "pw", u.Password)
	assert.Equal(t, "user", u.Role)
	assert.Equal(t, UserStatusActive, u.Status)
	assert.False(t, u.IsAdmin())
	assert.False(t, u.IsDeleted())
	assert.False(t, u.CreatedAt.IsZero())
	assert.False(t, u.UpdatedAt.IsZero())
	assert.Nil(t, u.DeletedAt)
}

func TestUser_NewAdminAndIsAdmin(t *testing.T) {
	u := NewAdmin("admin", "pw")
	assert.NotEmpty(t, u.UUID)
	assert.Equal(t, "admin", u.Role)
	assert.Equal(t, UserStatusActive, u.Status)
	assert.True(t, u.IsAdmin())
	assert.False(t, u.CreatedAt.IsZero())
}

func TestUser_SoftDelete(t *testing.T) {
	u := NewUser("alice", "pw")
	u.ID = 7
	beforeUUID := u.UUID

	u.SoftDelete()

	assert.Equal(t, beforeUUID, u.UUID)
	assert.Equal(t, "deleted-user-7", u.Name)
	assert.Equal(t, "", u.Email)
	assert.Equal(t, "", u.Password)
	assert.Equal(t, UserStatusDeleted, u.Status)
	assert.True(t, u.IsDeleted())
	assert.NotNil(t, u.DeletedAt)
	assert.False(t, u.UpdatedAt.IsZero())
}
