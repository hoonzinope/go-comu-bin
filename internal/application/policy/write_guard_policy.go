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
