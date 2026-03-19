package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type UserStatus string
type GuestStatus string

const (
	UserStatusActive    UserStatus = "active"
	UserStatusSuspended UserStatus = "suspended"
	UserStatusDeleted   UserStatus = "deleted"

	GuestStatusPending GuestStatus = "pending"
	GuestStatusActive  GuestStatus = "active"
	GuestStatusExpired GuestStatus = "expired"
)

type User struct {
	ID               int64
	UUID             string
	Name             string
	Email            string
	Password         string
	Guest            bool
	GuestStatus      GuestStatus
	GuestIssuedAt    *time.Time
	GuestActivatedAt *time.Time
	GuestExpiredAt   *time.Time
	Role             string
	Status           UserStatus
	SuspensionReason string
	SuspendedUntil   *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        *time.Time
}

func (u *User) IsAdmin() bool {
	return u.Role == "admin"
}

func (u *User) IsGuest() bool {
	return u.Guest
}

func (u *User) IsActiveGuest() bool {
	return u.Guest && u.GuestStatus == GuestStatusActive
}

func (u *User) IsDeleted() bool {
	return u.Status == UserStatusDeleted
}

func (u *User) IsSuspended() bool {
	if u.Status != UserStatusSuspended {
		return false
	}
	if u.SuspendedUntil == nil {
		return true
	}
	return u.SuspendedUntil.After(time.Now())
}

func (u *User) Suspend(reason string, until *time.Time) {
	u.Status = UserStatusSuspended
	u.SuspensionReason = reason
	u.SuspendedUntil = until
	u.UpdatedAt = time.Now()
}

func (u *User) Unsuspend() {
	u.Status = UserStatusActive
	u.SuspensionReason = ""
	u.SuspendedUntil = nil
	u.UpdatedAt = time.Now()
}

func (u *User) SoftDelete() {
	now := time.Now()
	u.Name = fmt.Sprintf("deleted-user-%d", u.ID)
	u.Email = ""
	u.Password = ""
	u.Guest = false
	u.GuestStatus = ""
	u.GuestIssuedAt = nil
	u.GuestActivatedAt = nil
	u.GuestExpiredAt = nil
	u.Status = UserStatusDeleted
	u.SuspensionReason = ""
	u.SuspendedUntil = nil
	u.UpdatedAt = now
	u.DeletedAt = &now
}

func NewUser(name, password string) *User {
	now := time.Now()
	return &User{
		UUID:      uuid.NewString(),
		Name:      name,
		Email:     "",
		Password:  password,
		Guest:     false,
		Role:      "user",
		Status:    UserStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func NewGuest(name, email, password string) *User {
	now := time.Now()
	return &User{
		UUID:          uuid.NewString(),
		Name:          name,
		Email:         email,
		Password:      password,
		Guest:         true,
		GuestStatus:   GuestStatusPending,
		GuestIssuedAt: &now,
		Role:          "user",
		Status:        UserStatusActive,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func NewAdmin(name, password string) *User {
	now := time.Now()
	return &User{
		UUID:      uuid.NewString(),
		Name:      name,
		Email:     "",
		Password:  password,
		Guest:     false,
		Role:      "admin",
		Status:    UserStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (u *User) UpgradeGuest(name, email, password string) {
	u.Name = name
	u.Email = email
	u.Password = password
	u.Guest = false
	u.GuestStatus = ""
	u.GuestIssuedAt = nil
	u.GuestActivatedAt = nil
	u.GuestExpiredAt = nil
	u.UpdatedAt = time.Now()
}

func (u *User) MarkGuestActive() {
	now := time.Now()
	u.GuestStatus = GuestStatusActive
	u.GuestActivatedAt = &now
	u.GuestExpiredAt = nil
	u.UpdatedAt = now
}

func (u *User) MarkGuestExpired() {
	now := time.Now()
	u.GuestStatus = GuestStatusExpired
	u.GuestExpiredAt = &now
	u.UpdatedAt = now
}
