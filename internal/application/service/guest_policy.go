package service

import (
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

func forbidGuest(user *entity.User) error {
	if user != nil && user.IsGuest() {
		return customerror.ErrForbidden
	}
	return nil
}

func ensureGuestLifecycleAllowsWrite(user *entity.User) error {
	if user == nil {
		return customerror.ErrUnauthorized
	}
	if user.IsGuest() && !user.IsActiveGuest() {
		return customerror.ErrForbidden
	}
	return nil
}
