package policy

import (
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

func ForbidGuest(user *entity.User) error {
	if user != nil && user.IsGuest() {
		return customerror.ErrForbidden
	}
	return nil
}

func EnsureGuestLifecycleAllowsWrite(user *entity.User) error {
	if user == nil {
		return customerror.ErrUnauthorized
	}
	if user.IsGuest() && !user.IsActiveGuest() {
		return customerror.ErrForbidden
	}
	return nil
}

func RequireVerifiedEmail(user *entity.User) error {
	if user == nil {
		return customerror.ErrUnauthorized
	}
	if user.IsAdmin() {
		return nil
	}
	if user.Email == "" || !user.IsEmailVerified() {
		return customerror.ErrEmailVerificationRequired
	}
	return nil
}
