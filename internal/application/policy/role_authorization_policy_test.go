package policy

import (
	"errors"
	"testing"
	"time"

	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
)

func TestRoleAuthorizationPolicy_AdminOnly(t *testing.T) {
	p := NewRoleAuthorizationPolicy()

	assert.NoError(t, p.AdminOnly(&entity.User{Role: "admin"}))
	assert.True(t, errors.Is(p.AdminOnly(&entity.User{Role: "user"}), customError.ErrForbidden))
	assert.True(t, errors.Is(p.AdminOnly(nil), customError.ErrUnauthorized))
}

func TestRoleAuthorizationPolicy_OwnerOrAdmin(t *testing.T) {
	p := NewRoleAuthorizationPolicy()

	assert.NoError(t, p.OwnerOrAdmin(&entity.User{ID: 7, Role: "user"}, 7))
	assert.NoError(t, p.OwnerOrAdmin(&entity.User{ID: 1, Role: "admin"}, 7))
	assert.True(t, errors.Is(p.OwnerOrAdmin(&entity.User{ID: 1, Role: "user"}, 7), customError.ErrForbidden))
	assert.True(t, errors.Is(p.OwnerOrAdmin(nil, 7), customError.ErrUnauthorized))
}

func TestRoleAuthorizationPolicy_CanWrite(t *testing.T) {
	p := NewRoleAuthorizationPolicy()
	until := time.Now().Add(time.Hour)

	assert.NoError(t, p.CanWrite(&entity.User{Status: entity.UserStatusActive}))
	assert.True(t, errors.Is(p.CanWrite(&entity.User{Status: entity.UserStatusSuspended, SuspendedUntil: &until}), customError.ErrUserSuspended))
	assert.True(t, errors.Is(p.CanWrite(nil), customError.ErrUnauthorized))
}
