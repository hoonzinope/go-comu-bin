package port

import "github.com/hoonzinope/go-comu-bin/internal/domain/entity"

type UserUseCase interface {
	SignUp(username, password string) (string, error)
	DeleteMe(userID int64, password string) error
	SuspendUser(adminID, targetUserID int64, reason string, duration entity.SuspensionDuration) error
	UnsuspendUser(adminID, targetUserID int64) error
}
