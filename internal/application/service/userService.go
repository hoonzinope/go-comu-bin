package service

import (
	"errors"
	"strings"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.UserUseCase = (*UserService)(nil)
var _ port.CredentialVerifier = (*UserService)(nil)

type UserService struct {
	userRepository      port.UserRepository
	passwordHasher      port.PasswordHasher
	authorizationPolicy policy.AuthorizationPolicy
}

func NewUserService(userRepository port.UserRepository, passwordHasher port.PasswordHasher, authorizationPolicies ...policy.AuthorizationPolicy) *UserService {
	var authorizationPolicy policy.AuthorizationPolicy = policy.NewRoleAuthorizationPolicy()
	if len(authorizationPolicies) > 0 && authorizationPolicies[0] != nil {
		authorizationPolicy = authorizationPolicies[0]
	}
	return &UserService{
		userRepository:      userRepository,
		passwordHasher:      passwordHasher,
		authorizationPolicy: authorizationPolicy,
	}
}

func (s *UserService) SignUp(username, password string) (string, error) {
	// 회원가입 로직 구현
	// duplicate username check
	if strings.TrimSpace(username) == "" || strings.TrimSpace(password) == "" {
		return "", customError.ErrInvalidInput
	}
	existingUser, err := s.userRepository.SelectUserByUsername(username)
	if err != nil {
		return "", customError.WrapRepository("select user by username for signup", err)
	}
	if existingUser != nil {
		return "", customError.ErrUserAlreadyExists
	}

	hashedPassword, err := s.passwordHasher.Hash(password)
	if err != nil {
		return "", customError.Wrap(customError.ErrInternalServerError, "hash password for signup", err)
	}
	newUser := entity.NewUser(username, hashedPassword)

	_, err = s.userRepository.Save(newUser)
	if err != nil {
		if errors.Is(err, customError.ErrUserAlreadyExists) {
			return "", customError.ErrUserAlreadyExists
		}
		return "", customError.WrapRepository("save user for signup", err)
	}
	return "ok", nil
}

func (s *UserService) DeleteMe(userID int64, password string) error {
	existingUser, err := s.userRepository.SelectUserByID(userID)
	if err != nil {
		return customError.WrapRepository("select user by id for delete me", err)
	}
	if existingUser == nil {
		return customError.ErrUserNotFound
	}
	matched, err := s.passwordHasher.Matches(existingUser.Password, password)
	if err != nil {
		return customError.Wrap(customError.ErrInternalServerError, "compare password for delete me", err)
	}
	if !matched {
		return customError.ErrInvalidCredential
	}
	existingUser.SoftDelete()
	err = s.userRepository.Update(existingUser)
	if err != nil {
		return customError.WrapRepository("soft delete user for delete me", err)
	}
	return nil
}

func (s *UserService) VerifyCredentials(username, password string) (int64, error) {
	existingUser, err := s.userRepository.SelectUserByUsername(username)
	if err != nil {
		return 0, customError.WrapRepository("select user by username for verify credentials", err)
	}
	if existingUser == nil {
		return 0, customError.ErrInvalidCredential
	}

	matched, err := s.passwordHasher.Matches(existingUser.Password, password)
	if err != nil {
		return 0, customError.Wrap(customError.ErrInternalServerError, "compare password for verify credentials", err)
	}
	if !matched {
		return 0, customError.ErrInvalidCredential
	}
	return existingUser.ID, nil
}

func (s *UserService) SuspendUser(adminID, targetUserID int64, reason string, duration entity.SuspensionDuration) error {
	if strings.TrimSpace(reason) == "" {
		return customError.ErrInvalidInput
	}
	until, ok := duration.EndTime(time.Now())
	if !ok {
		return customError.ErrInvalidInput
	}
	admin, err := s.userRepository.SelectUserByID(adminID)
	if err != nil {
		return customError.WrapRepository("select admin by id for suspend user", err)
	}
	if admin == nil {
		return customError.ErrUserNotFound
	}
	if err := s.authorizationPolicy.AdminOnly(admin); err != nil {
		return err
	}
	target, err := s.userRepository.SelectUserByID(targetUserID)
	if err != nil {
		return customError.WrapRepository("select target user by id for suspend user", err)
	}
	if target == nil {
		return customError.ErrUserNotFound
	}
	target.Suspend(strings.TrimSpace(reason), until)
	if err := s.userRepository.Update(target); err != nil {
		return customError.WrapRepository("update user suspension", err)
	}
	return nil
}

func (s *UserService) GetUserSuspension(adminID, targetUserID int64) (*model.UserSuspension, error) {
	admin, err := s.userRepository.SelectUserByID(adminID)
	if err != nil {
		return nil, customError.WrapRepository("select admin by id for get user suspension", err)
	}
	if admin == nil {
		return nil, customError.ErrUserNotFound
	}
	if err := s.authorizationPolicy.AdminOnly(admin); err != nil {
		return nil, err
	}
	target, err := s.userRepository.SelectUserByID(targetUserID)
	if err != nil {
		return nil, customError.WrapRepository("select target user by id for get user suspension", err)
	}
	if target == nil {
		return nil, customError.ErrUserNotFound
	}
	if target.Status == entity.UserStatusSuspended && !target.IsSuspended() {
		target.Unsuspend()
		if err := s.userRepository.Update(target); err != nil {
			return nil, customError.WrapRepository("refresh expired user suspension", err)
		}
	}
	return &model.UserSuspension{
		UserID:         target.ID,
		Status:         target.Status,
		Reason:         target.SuspensionReason,
		SuspendedUntil: target.SuspendedUntil,
	}, nil
}

func (s *UserService) UnsuspendUser(adminID, targetUserID int64) error {
	admin, err := s.userRepository.SelectUserByID(adminID)
	if err != nil {
		return customError.WrapRepository("select admin by id for unsuspend user", err)
	}
	if admin == nil {
		return customError.ErrUserNotFound
	}
	if err := s.authorizationPolicy.AdminOnly(admin); err != nil {
		return err
	}
	target, err := s.userRepository.SelectUserByID(targetUserID)
	if err != nil {
		return customError.WrapRepository("select target user by id for unsuspend user", err)
	}
	if target == nil {
		return customError.ErrUserNotFound
	}
	target.Unsuspend()
	if err := s.userRepository.Update(target); err != nil {
		return customError.WrapRepository("clear user suspension", err)
	}
	return nil
}
