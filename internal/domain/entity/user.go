package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type UserStatus string

const (
	UserStatusActive    UserStatus = "active"
	UserStatusSuspended UserStatus = "suspended"
	UserStatusDeleted   UserStatus = "deleted"
)

type User struct {
	ID               int64
	UUID             string
	Name             string
	Email            string
	Password         string
	Guest            bool
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
		UUID:      uuid.NewString(),
		Name:      name,
		Email:     email,
		Password:  password,
		Guest:     true,
		Role:      "user",
		Status:    UserStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
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
	u.UpdatedAt = time.Now()
}
