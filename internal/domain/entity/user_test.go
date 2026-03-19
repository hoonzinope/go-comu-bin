package entity

import (
	"testing"
	"time"

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
	assert.False(t, u.IsSuspended())
	assert.False(t, u.CreatedAt.IsZero())
	assert.False(t, u.UpdatedAt.IsZero())
	assert.Nil(t, u.DeletedAt)
	assert.Nil(t, u.SuspendedUntil)
	assert.Equal(t, "", u.SuspensionReason)
}

func TestUser_NewAdminAndIsAdmin(t *testing.T) {
	u := NewAdmin("admin", "pw")
	assert.NotEmpty(t, u.UUID)
	assert.Equal(t, "admin", u.Role)
	assert.Equal(t, UserStatusActive, u.Status)
	assert.True(t, u.IsAdmin())
	assert.False(t, u.CreatedAt.IsZero())
}

func TestUser_NewGuest_TracksGuestLifecycle(t *testing.T) {
	u := NewGuest("guest-1", "guest-1@example.invalid", "pw")

	assert.True(t, u.IsGuest())
	assert.Equal(t, GuestStatusPending, u.GuestStatus)
	assert.NotNil(t, u.GuestIssuedAt)
	assert.Nil(t, u.GuestActivatedAt)
	assert.Nil(t, u.GuestExpiredAt)
}

func TestUser_GuestLifecycleTransitions(t *testing.T) {
	u := NewGuest("guest-1", "guest-1@example.invalid", "pw")

	u.MarkGuestActive()
	assert.Equal(t, GuestStatusActive, u.GuestStatus)
	assert.NotNil(t, u.GuestActivatedAt)
	assert.Nil(t, u.GuestExpiredAt)

	u.MarkGuestExpired()
	assert.Equal(t, GuestStatusExpired, u.GuestStatus)
	assert.NotNil(t, u.GuestExpiredAt)
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
	assert.Equal(t, GuestStatus(""), u.GuestStatus)
	assert.Nil(t, u.GuestIssuedAt)
	assert.Nil(t, u.GuestActivatedAt)
	assert.Nil(t, u.GuestExpiredAt)
}

func TestUser_SuspendUnlimited(t *testing.T) {
	u := NewUser("alice", "pw")

	u.Suspend("spam", nil)

	assert.Equal(t, UserStatusSuspended, u.Status)
	assert.True(t, u.IsSuspended())
	assert.Equal(t, "spam", u.SuspensionReason)
	assert.Nil(t, u.SuspendedUntil)
}

func TestUser_SuspendUntilFuture(t *testing.T) {
	u := NewUser("alice", "pw")
	until := time.Now().Add(24 * time.Hour)

	u.Suspend("spam", &until)

	assert.True(t, u.IsSuspended())
	assert.Equal(t, &until, u.SuspendedUntil)
}

func TestUser_Unsuspend(t *testing.T) {
	u := NewUser("alice", "pw")
	u.Suspend("spam", nil)

	u.Unsuspend()

	assert.Equal(t, UserStatusActive, u.Status)
	assert.False(t, u.IsSuspended())
	assert.Equal(t, "", u.SuspensionReason)
	assert.Nil(t, u.SuspendedUntil)
}

func TestUser_UpgradeGuest_ClearsGuestLifecycle(t *testing.T) {
	u := NewGuest("guest-1", "guest-1@example.invalid", "pw")
	u.MarkGuestActive()

	u.UpgradeGuest("alice", "alice@example.com", "hashed")

	assert.False(t, u.IsGuest())
	assert.Equal(t, GuestStatus(""), u.GuestStatus)
	assert.Nil(t, u.GuestIssuedAt)
	assert.Nil(t, u.GuestActivatedAt)
	assert.Nil(t, u.GuestExpiredAt)
}
