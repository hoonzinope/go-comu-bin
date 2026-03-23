package port

import "context"

import "github.com/hoonzinope/go-comu-bin/internal/application/model"

type UserUseCase interface {
	SignUp(ctx context.Context, username, email, password string) (string, error)
	IssueGuestAccount(ctx context.Context) (int64, error)
	UpgradeGuest(ctx context.Context, userID int64, username, email, password string) error
	DeleteMe(ctx context.Context, userID int64, password string) error
	GetUserSuspension(ctx context.Context, adminID int64, targetUserUUID string) (*model.UserSuspension, error)
	SuspendUser(ctx context.Context, adminID int64, targetUserUUID, reason string, duration model.SuspensionDuration) error
	UnsuspendUser(ctx context.Context, adminID int64, targetUserUUID string) error
}
