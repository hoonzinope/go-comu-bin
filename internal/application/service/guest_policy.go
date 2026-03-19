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
