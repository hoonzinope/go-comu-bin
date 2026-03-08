package policy

import (
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ AuthorizationPolicy = (*RoleAuthorizationPolicy)(nil)

type RoleAuthorizationPolicy struct{}

func NewRoleAuthorizationPolicy() *RoleAuthorizationPolicy {
	return &RoleAuthorizationPolicy{}
}

func (p *RoleAuthorizationPolicy) AdminOnly(user *entity.User) error {
	if user == nil {
		return customError.ErrUnauthorized
	}
	if !user.IsAdmin() {
		return customError.ErrForbidden
	}
	return nil
}

func (p *RoleAuthorizationPolicy) OwnerOrAdmin(user *entity.User, resourceOwnerID int64) error {
	if user == nil {
		return customError.ErrUnauthorized
	}
	if user.ID != resourceOwnerID && !user.IsAdmin() {
		return customError.ErrForbidden
	}
	return nil
}

func (p *RoleAuthorizationPolicy) CanWrite(user *entity.User) error {
	if user == nil {
		return customError.ErrUnauthorized
	}
	if user.IsSuspended() {
		return customError.ErrUserSuspended
	}
	return nil
}
