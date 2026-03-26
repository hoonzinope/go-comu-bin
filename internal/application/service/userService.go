package service

import (
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	usersvc "github.com/hoonzinope/go-comu-bin/internal/application/service/user"
)

func NewUserService(userRepository port.UserRepository, passwordHasher port.PasswordHasher, unitOfWork port.UnitOfWork, authorizationPolicies ...policy.AuthorizationPolicy) *usersvc.UserService {
	return usersvc.NewUserService(userRepository, passwordHasher, unitOfWork, authorizationPolicies...)
}

func NewUserServiceWithEmailVerification(userRepository port.UserRepository, passwordHasher port.PasswordHasher, unitOfWork port.UnitOfWork, verificationTokens port.EmailVerificationTokenRepository, verificationIssuer port.EmailVerificationTokenIssuer, verificationMailer port.EmailVerificationMailSender, verificationTokenTTL time.Duration, authorizationPolicies ...policy.AuthorizationPolicy) *usersvc.UserService {
	return usersvc.NewUserServiceWithEmailVerification(userRepository, passwordHasher, unitOfWork, verificationTokens, verificationIssuer, verificationMailer, verificationTokenTTL, authorizationPolicies...)
}
