package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type UserStatus string

const (
	UserStatusActive  UserStatus = "active"
	UserStatusDeleted UserStatus = "deleted"
)

type User struct {
	ID        int64
	UUID      string
	Name      string
	Email     string
	Password  string
	Role      string
	Status    UserStatus
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

func (u *User) IsAdmin() bool {
	return u.Role == "admin"
}

func (u *User) IsDeleted() bool {
	return u.Status == UserStatusDeleted
}

func (u *User) SoftDelete() {
	now := time.Now()
	u.Name = fmt.Sprintf("deleted-user-%d", u.ID)
	u.Email = ""
	u.Password = ""
	u.Status = UserStatusDeleted
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
		Role:      "admin",
		Status:    UserStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
