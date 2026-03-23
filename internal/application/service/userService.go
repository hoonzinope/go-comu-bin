package service

import (
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	usersvc "github.com/hoonzinope/go-comu-bin/internal/application/service/user"
)

type UserService = usersvc.Service

func NewUserService(userRepository port.UserRepository, passwordHasher port.PasswordHasher, unitOfWork port.UnitOfWork, authorizationPolicies ...policy.AuthorizationPolicy) *UserService {
	return usersvc.NewService(userRepository, passwordHasher, unitOfWork, authorizationPolicies...)
}

func NewUserServiceWithEmailVerification(userRepository port.UserRepository, passwordHasher port.PasswordHasher, unitOfWork port.UnitOfWork, verificationTokens port.EmailVerificationTokenRepository, verificationIssuer port.EmailVerificationTokenIssuer, verificationMailer port.EmailVerificationMailSender, verificationTokenTTL time.Duration, authorizationPolicies ...policy.AuthorizationPolicy) *UserService {
	return usersvc.NewUserServiceWithEmailVerification(userRepository, passwordHasher, unitOfWork, verificationTokens, verificationIssuer, verificationMailer, verificationTokenTTL, authorizationPolicies...)
}
