package model

import (
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type UserSuspension struct {
	UserID         int64
	Status         entity.UserStatus
	Reason         string
	SuspendedUntil *time.Time
}
