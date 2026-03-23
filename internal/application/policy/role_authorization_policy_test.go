package policy

import (
	"errors"
	"testing"
	"time"

	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
)

func TestRoleAuthorizationPolicy_AdminOnly(t *testing.T) {
	p := NewRoleAuthorizationPolicy()

	assert.NoError(t, p.AdminOnly(&entity.User{Role: "admin"}))
	assert.True(t, errors.Is(p.AdminOnly(&entity.User{Role: "user"}), customerror.ErrForbidden))
	assert.True(t, errors.Is(p.AdminOnly(nil), customerror.ErrUnauthorized))
}

func TestRoleAuthorizationPolicy_OwnerOrAdmin(t *testing.T) {
	p := NewRoleAuthorizationPolicy()

	assert.NoError(t, p.OwnerOrAdmin(&entity.User{ID: 7, Role: "user"}, 7))
	assert.NoError(t, p.OwnerOrAdmin(&entity.User{ID: 1, Role: "admin"}, 7))
	assert.True(t, errors.Is(p.OwnerOrAdmin(&entity.User{ID: 1, Role: "user"}, 7), customerror.ErrForbidden))
	assert.True(t, errors.Is(p.OwnerOrAdmin(nil, 7), customerror.ErrUnauthorized))
}

func TestRoleAuthorizationPolicy_CanWrite(t *testing.T) {
	p := NewRoleAuthorizationPolicy()
	until := time.Now().Add(time.Hour)

	assert.NoError(t, p.CanWrite(&entity.User{Status: entity.UserStatusActive}))
	assert.True(t, errors.Is(p.CanWrite(&entity.User{Status: entity.UserStatusSuspended, SuspendedUntil: &until}), customerror.ErrUserSuspended))
	assert.True(t, errors.Is(p.CanWrite(nil), customerror.ErrUnauthorized))
}

func TestRequireVerifiedEmail(t *testing.T) {
	verifiedAt := time.Now()

	assert.NoError(t, RequireVerifiedEmail(&entity.User{Role: "admin"}))
	assert.NoError(t, RequireVerifiedEmail(&entity.User{Email: "alice@example.com", EmailVerifiedAt: &verifiedAt}))
	assert.True(t, errors.Is(RequireVerifiedEmail(&entity.User{Email: "alice@example.com"}), customerror.ErrEmailVerificationRequired))
	assert.True(t, errors.Is(RequireVerifiedEmail(&entity.User{}), customerror.ErrEmailVerificationRequired))
	assert.True(t, errors.Is(RequireVerifiedEmail(nil), customerror.ErrUnauthorized))
}
