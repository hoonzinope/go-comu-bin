package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubUserUseCase struct {
	deleteMe func(ctx context.Context, userID int64, password string) error
}

func (s *stubUserUseCase) SignUp(ctx context.Context, username, password string) (string, error) {
	return "ok", nil
}

func (s *stubUserUseCase) DeleteMe(ctx context.Context, userID int64, password string) error {
	if s.deleteMe != nil {
		return s.deleteMe(ctx, userID, password)
	}
	return nil
}

func (s *stubUserUseCase) GetUserSuspension(ctx context.Context, adminID int64, targetUserUUID string) (*model.UserSuspension, error) {
	return &model.UserSuspension{
		UserUUID:       targetUserUUID,
		Status:         entity.UserStatusActive,
		SuspendedUntil: nil,
	}, nil
}

func (s *stubUserUseCase) SuspendUser(ctx context.Context, adminID int64, targetUserUUID, reason string, duration entity.SuspensionDuration) error {
	return nil
}

func (s *stubUserUseCase) UnsuspendUser(ctx context.Context, adminID int64, targetUserUUID string) error {
	return nil
}

type stubSessionUseCase struct {
	invalidateUserSessions func(ctx context.Context, userID int64) error
}

func (s *stubSessionUseCase) Login(ctx context.Context, username, password string) (string, error) {
	return "", nil
}

func (s *stubSessionUseCase) Logout(ctx context.Context, token string) error {
	return nil
}

func (s *stubSessionUseCase) InvalidateUserSessions(ctx context.Context, userID int64) error {
	if s.invalidateUserSessions != nil {
		return s.invalidateUserSessions(ctx, userID)
	}
	return nil
}

func (s *stubSessionUseCase) ValidateTokenToId(ctx context.Context, token string) (int64, error) {
	return 0, nil
}

func TestAccountService_DeleteMyAccount_Success(t *testing.T) {
	calledDeleteMe := false
	calledInvalidate := false
	svc := NewAccountService(
		&stubUserUseCase{
			deleteMe: func(ctx context.Context, userID int64, password string) error {
				calledDeleteMe = true
				assert.Equal(t, int64(10), userID)
				assert.Equal(t, "pw", password)
				return nil
			},
		},
		&stubSessionUseCase{
			invalidateUserSessions: func(ctx context.Context, userID int64) error {
				calledInvalidate = true
				assert.Equal(t, int64(10), userID)
				return nil
			},
		},
	)

	require.NoError(t, svc.DeleteMyAccount(context.Background(), 10, "pw"))
	assert.True(t, calledDeleteMe)
	assert.True(t, calledInvalidate)
}

func TestAccountService_DeleteMyAccount_StopsOnDeleteFailure(t *testing.T) {
	calledInvalidate := false
	svc := NewAccountService(
		&stubUserUseCase{
			deleteMe: func(ctx context.Context, userID int64, password string) error {
				return customError.ErrInvalidCredential
			},
		},
		&stubSessionUseCase{
			invalidateUserSessions: func(ctx context.Context, userID int64) error {
				calledInvalidate = true
				return nil
			},
		},
	)

	err := svc.DeleteMyAccount(context.Background(), 10, "bad")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidCredential))
	assert.False(t, calledInvalidate)
}

func TestAccountService_DeleteMyAccount_IgnoresSessionInvalidationFailure(t *testing.T) {
	originalLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(func() {
		slog.SetDefault(originalLogger)
	})

	calledInvalidate := false
	svc := NewAccountService(
		&stubUserUseCase{
			deleteMe: func(ctx context.Context, userID int64, password string) error {
				return nil
			},
		},
		&stubSessionUseCase{
			invalidateUserSessions: func(ctx context.Context, userID int64) error {
				calledInvalidate = true
				return customError.WrapRepository("delete sessions", errors.New("cache unavailable"))
			},
		},
	)

	err := svc.DeleteMyAccount(context.Background(), 10, "pw")
	require.NoError(t, err)
	assert.True(t, calledInvalidate)
}
