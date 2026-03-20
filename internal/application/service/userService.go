package service

import (
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	usersvc "github.com/hoonzinope/go-comu-bin/internal/application/service/user"
)

type UserService = usersvc.Service

func NewUserService(userRepository port.UserRepository, passwordHasher port.PasswordHasher, unitOfWork port.UnitOfWork, authorizationPolicies ...policy.AuthorizationPolicy) *UserService {
	return usersvc.NewService(userRepository, passwordHasher, unitOfWork, authorizationPolicies...)
}
