package model

import (
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type User struct {
	ID              int64
	UUID            string
	Name            string
	Email           string
	Guest           bool
	GuestStatus     entity.GuestStatus
	EmailVerifiedAt *time.Time
	Role            string
	Status          entity.UserStatus
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
