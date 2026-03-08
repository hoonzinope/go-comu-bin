package port

import "github.com/hoonzinope/go-comu-bin/internal/domain/entity"
import "github.com/hoonzinope/go-comu-bin/internal/application/model"

type UserUseCase interface {
	SignUp(username, password string) (string, error)
	DeleteMe(userID int64, password string) error
	GetUserSuspension(adminID int64, targetUserUUID string) (*model.UserSuspension, error)
	SuspendUser(adminID int64, targetUserUUID, reason string, duration entity.SuspensionDuration) error
	UnsuspendUser(adminID int64, targetUserUUID string) error
}
