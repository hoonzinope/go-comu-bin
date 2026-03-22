package common

import (
	"context"
	"strings"

	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

func RequireAdminUser(ctx context.Context, userRepository port.UserRepository, authorizationPolicy policy.AuthorizationPolicy, userID int64, operation string) (*entity.User, error) {
	admin, err := userRepository.SelectUserByID(ctx, userID)
	if err != nil {
		op := strings.TrimSpace(operation)
		if op == "" {
			op = "admin operation"
		}
		return nil, customerror.WrapRepository("select admin by id for "+op, err)
	}
	if admin == nil {
		return nil, customerror.ErrUserNotFound
	}
	if err := authorizationPolicy.AdminOnly(admin); err != nil {
		return nil, err
	}
	return admin, nil
}
