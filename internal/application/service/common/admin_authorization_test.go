package common

import (
	"context"
	"errors"
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/hoonzinope/go-comu-bin/internal/infrastructure/persistence/inmemory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type adminGuardUserRepository struct {
	port.UserRepository
	selectUserByID func(ctx context.Context, id int64) (*entity.User, error)
}

func (r adminGuardUserRepository) SelectUserByID(ctx context.Context, id int64) (*entity.User, error) {
	if r.selectUserByID != nil {
		return r.selectUserByID(ctx, id)
	}
	return r.UserRepository.SelectUserByID(ctx, id)
}

func TestRequireAdminUser_Success(t *testing.T) {
	repo := inmemory.NewUserRepository()
	admin := entity.NewAdmin("admin", "hashed")
	adminID, err := repo.Save(context.Background(), admin)
	require.NoError(t, err)

	got, err := RequireAdminUser(context.Background(), repo, policy.NewRoleAuthorizationPolicy(), adminID, "board admin")

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, adminID, got.ID)
	assert.True(t, got.IsAdmin())
}

func TestRequireAdminUser_RepoError(t *testing.T) {
	repo := adminGuardUserRepository{
		UserRepository: inmemory.NewUserRepository(),
		selectUserByID: func(ctx context.Context, id int64) (*entity.User, error) {
			return nil, errors.New("db down")
		},
	}

	got, err := RequireAdminUser(context.Background(), repo, policy.NewRoleAuthorizationPolicy(), 1, "report admin")

	assert.Nil(t, got)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrRepositoryFailure))
	assert.Contains(t, err.Error(), "select admin by id for report admin")
}

func TestRequireAdminUser_UserNotFound(t *testing.T) {
	repo := inmemory.NewUserRepository()

	got, err := RequireAdminUser(context.Background(), repo, policy.NewRoleAuthorizationPolicy(), 999, "outbox admin")

	assert.Nil(t, got)
	assert.True(t, errors.Is(err, customerror.ErrUserNotFound))
}

func TestRequireAdminUser_ForbiddenForNonAdmin(t *testing.T) {
	repo := inmemory.NewUserRepository()
	user := entity.NewUser("user", "hashed")
	userID, err := repo.Save(context.Background(), user)
	require.NoError(t, err)

	got, err := RequireAdminUser(context.Background(), repo, policy.NewRoleAuthorizationPolicy(), userID, "user suspension")

	assert.Nil(t, got)
	assert.True(t, errors.Is(err, customerror.ErrForbidden))
}
