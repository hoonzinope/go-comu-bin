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
	unitOfWork          port.UnitOfWork
	authorizationPolicy policy.AuthorizationPolicy
}

func NewUserService(userRepository port.UserRepository, passwordHasher port.PasswordHasher, unitOfWork port.UnitOfWork, authorizationPolicies ...policy.AuthorizationPolicy) *UserService {
	var authorizationPolicy policy.AuthorizationPolicy = policy.NewRoleAuthorizationPolicy()
	if len(authorizationPolicies) > 0 && authorizationPolicies[0] != nil {
		authorizationPolicy = authorizationPolicies[0]
	}
	return &UserService{
		userRepository:      userRepository,
		passwordHasher:      passwordHasher,
		unitOfWork:          unitOfWork,
		authorizationPolicy: authorizationPolicy,
	}
}

func (s *UserService) SignUp(username, password string) (string, error) {
	// 회원가입 로직 구현
	// duplicate username check
	if strings.TrimSpace(username) == "" || strings.TrimSpace(password) == "" {
		return "", customError.ErrInvalidInput
	}
	hashedPassword, err := s.passwordHasher.Hash(password)
	if err != nil {
		return "", customError.Wrap(customError.ErrInternalServerError, "hash password for signup", err)
	}
	newUser := entity.NewUser(username, hashedPassword)

	err = s.unitOfWork.WithinTransaction(func(tx port.TxScope) error {
		existingUser, repoErr := tx.UserRepository().SelectUserByUsername(username)
		if repoErr != nil {
			return customError.WrapRepository("select user by username for signup", repoErr)
		}
		if existingUser != nil {
			return customError.ErrUserAlreadyExists
		}
		_, repoErr = tx.UserRepository().Save(newUser)
		if repoErr != nil {
			if errors.Is(repoErr, customError.ErrUserAlreadyExists) {
				return customError.ErrUserAlreadyExists
			}
			return customError.WrapRepository("save user for signup", repoErr)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return "ok", nil
}

func (s *UserService) DeleteMe(userID int64, password string) error {
	return s.unitOfWork.WithinTransaction(func(tx port.TxScope) error {
		existingUser, err := tx.UserRepository().SelectUserByID(userID)
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
		if err := tx.UserRepository().Update(existingUser); err != nil {
			return customError.WrapRepository("soft delete user for delete me", err)
		}
		return nil
	})
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

func (s *UserService) SuspendUser(adminID int64, targetUserUUID, reason string, duration entity.SuspensionDuration) error {
	if strings.TrimSpace(reason) == "" {
		return customError.ErrInvalidInput
	}
	until, ok := duration.EndTime(time.Now())
	if !ok {
		return customError.ErrInvalidInput
	}
	return s.unitOfWork.WithinTransaction(func(tx port.TxScope) error {
		admin, err := tx.UserRepository().SelectUserByID(adminID)
		if err != nil {
			return customError.WrapRepository("select admin by id for suspend user", err)
		}
		if admin == nil {
			return customError.ErrUserNotFound
		}
		if err := s.authorizationPolicy.AdminOnly(admin); err != nil {
			return err
		}
		target, err := tx.UserRepository().SelectUserByUUID(targetUserUUID)
		if err != nil {
			return customError.WrapRepository("select target user by uuid for suspend user", err)
		}
		if target == nil {
			return customError.ErrUserNotFound
		}
		target.Suspend(strings.TrimSpace(reason), until)
		if err := tx.UserRepository().Update(target); err != nil {
			return customError.WrapRepository("update user suspension", err)
		}
		return nil
	})
}

func (s *UserService) GetUserSuspension(adminID int64, targetUserUUID string) (*model.UserSuspension, error) {
	var suspension *model.UserSuspension
	err := s.unitOfWork.WithinTransaction(func(tx port.TxScope) error {
		admin, err := tx.UserRepository().SelectUserByID(adminID)
		if err != nil {
			return customError.WrapRepository("select admin by id for get user suspension", err)
		}
		if admin == nil {
			return customError.ErrUserNotFound
		}
		if err := s.authorizationPolicy.AdminOnly(admin); err != nil {
			return err
		}
		target, err := tx.UserRepository().SelectUserByUUID(targetUserUUID)
		if err != nil {
			return customError.WrapRepository("select target user by uuid for get user suspension", err)
		}
		if target == nil {
			return customError.ErrUserNotFound
		}
		status := target.Status
		reason := target.SuspensionReason
		suspendedUntil := target.SuspendedUntil
		if status == entity.UserStatusSuspended && !target.IsSuspended() {
			status = entity.UserStatusActive
			reason = ""
			suspendedUntil = nil
		}
		suspension = &model.UserSuspension{
			UserUUID:       target.UUID,
			Status:         status,
			Reason:         reason,
			SuspendedUntil: suspendedUntil,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return suspension, nil
}

func (s *UserService) UnsuspendUser(adminID int64, targetUserUUID string) error {
	return s.unitOfWork.WithinTransaction(func(tx port.TxScope) error {
		admin, err := tx.UserRepository().SelectUserByID(adminID)
		if err != nil {
			return customError.WrapRepository("select admin by id for unsuspend user", err)
		}
		if admin == nil {
			return customError.ErrUserNotFound
		}
		if err := s.authorizationPolicy.AdminOnly(admin); err != nil {
			return err
		}
		target, err := tx.UserRepository().SelectUserByUUID(targetUserUUID)
		if err != nil {
			return customError.WrapRepository("select target user by uuid for unsuspend user", err)
		}
		if target == nil {
			return customError.ErrUserNotFound
		}
		target.Unsuspend()
		if err := tx.UserRepository().Update(target); err != nil {
			return customError.WrapRepository("clear user suspension", err)
		}
		return nil
	})
}
