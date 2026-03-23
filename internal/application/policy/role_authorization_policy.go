package policy

import (
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ AuthorizationPolicy = (*RoleAuthorizationPolicy)(nil)

type RoleAuthorizationPolicy struct{}

func NewRoleAuthorizationPolicy() *RoleAuthorizationPolicy {
	return &RoleAuthorizationPolicy{}
}

func (p *RoleAuthorizationPolicy) AdminOnly(user *entity.User) error {
	if user == nil {
		return customerror.ErrUnauthorized
	}
	if !user.IsAdmin() {
		return customerror.ErrForbidden
	}
	return nil
}

func (p *RoleAuthorizationPolicy) OwnerOrAdmin(user *entity.User, resourceOwnerID int64) error {
	if user == nil {
		return customerror.ErrUnauthorized
	}
	if user.ID != resourceOwnerID && !user.IsAdmin() {
		return customerror.ErrForbidden
	}
	return nil
}

func (p *RoleAuthorizationPolicy) CanWrite(user *entity.User) error {
	if user == nil {
		return customerror.ErrUnauthorized
	}
	if user.IsSuspended() {
		return customerror.ErrUserSuspended
	}
	if user.Email != "" && !user.IsAdmin() && !user.IsEmailVerified() {
		return customerror.ErrEmailVerificationRequired
	}
	return nil
}
