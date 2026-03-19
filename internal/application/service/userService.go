package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.UserUseCase = (*UserService)(nil)
var _ port.CredentialVerifier = (*UserService)(nil)
var _ port.AdminAuthorizer = (*UserService)(nil)

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

func (s *UserService) SignUp(ctx context.Context, username, password string) (string, error) {
	username = normalizeUsername(username)
	if username == "" || strings.TrimSpace(password) == "" {
		return "", customerror.ErrInvalidInput
	}
	hashedPassword, err := s.passwordHasher.Hash(password)
	if err != nil {
		return "", customerror.Wrap(customerror.ErrInternalServerError, "hash password for signup", err)
	}
	newUser := entity.NewUser(username, hashedPassword)

	err = s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		existingUser, repoErr := tx.UserRepository().SelectUserByUsername(txCtx, username)
		if repoErr != nil {
			return customerror.WrapRepository("select user by username for signup", repoErr)
		}
		if existingUser != nil {
			return customerror.ErrUserAlreadyExists
		}
		_, repoErr = tx.UserRepository().Save(txCtx, newUser)
		if repoErr != nil {
			if errors.Is(repoErr, customerror.ErrUserAlreadyExists) {
				return customerror.ErrUserAlreadyExists
			}
			return customerror.WrapRepository("save user for signup", repoErr)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return "ok", nil
}

func (s *UserService) IssueGuestAccount(ctx context.Context) (int64, error) {
	rawSecret := uuid.NewString()
	hashedPassword, err := s.passwordHasher.Hash(rawSecret)
	if err != nil {
		return 0, customerror.Wrap(customerror.ErrInternalServerError, "hash password for guest issue", err)
	}
	guestToken := uuid.NewString()
	newGuest := entity.NewGuest(
		fmt.Sprintf("guest-%s", guestToken),
		fmt.Sprintf("guest-%s@example.invalid", guestToken),
		hashedPassword,
	)

	var guestID int64
	err = s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		var repoErr error
		guestID, repoErr = tx.UserRepository().Save(txCtx, newGuest)
		if repoErr != nil {
			if errors.Is(repoErr, customerror.ErrUserAlreadyExists) {
				return customerror.ErrUserAlreadyExists
			}
			return customerror.WrapRepository("save user for guest issue", repoErr)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return guestID, nil
}

func (s *UserService) UpgradeGuest(ctx context.Context, userID int64, username, email, password string) error {
	username = normalizeUsername(username)
	email = strings.TrimSpace(email)
	if username == "" || email == "" || strings.TrimSpace(password) == "" {
		return customerror.ErrInvalidInput
	}
	hashedPassword, err := s.passwordHasher.Hash(password)
	if err != nil {
		return customerror.Wrap(customerror.ErrInternalServerError, "hash password for guest upgrade", err)
	}
	return s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		existingUser, err := tx.UserRepository().SelectUserByID(txCtx, userID)
		if err != nil {
			return customerror.WrapRepository("select user by id for guest upgrade", err)
		}
		if existingUser == nil {
			return customerror.ErrUserNotFound
		}
		if !existingUser.IsGuest() {
			return customerror.ErrInvalidInput
		}
		upgradedUser := *existingUser
		upgradedUser.UpgradeGuest(username, email, hashedPassword)
		if err := tx.UserRepository().Update(txCtx, &upgradedUser); err != nil {
			if errors.Is(err, customerror.ErrUserAlreadyExists) {
				return customerror.ErrUserAlreadyExists
			}
			return customerror.WrapRepository("update user for guest upgrade", err)
		}
		return nil
	})
}

func (s *UserService) DeleteMe(ctx context.Context, userID int64, password string) error {
	return s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		existingUser, err := tx.UserRepository().SelectUserByID(txCtx, userID)
		if err != nil {
			return customerror.WrapRepository("select user by id for delete me", err)
		}
		if existingUser == nil {
			return customerror.ErrUserNotFound
		}
		if !existingUser.IsGuest() {
			matched, err := s.passwordHasher.Matches(existingUser.Password, password)
			if err != nil {
				return customerror.Wrap(customerror.ErrInternalServerError, "compare password for delete me", err)
			}
			if !matched {
				return customerror.ErrInvalidCredential
			}
		}
		existingUser.SoftDelete()
		if err := tx.UserRepository().Update(txCtx, existingUser); err != nil {
			return customerror.WrapRepository("soft delete user for delete me", err)
		}
		return nil
	})
}

func (s *UserService) VerifyCredentials(ctx context.Context, username, password string) (int64, error) {
	username = normalizeUsername(username)
	if username == "" {
		return 0, customerror.ErrInvalidCredential
	}
	existingUser, err := s.userRepository.SelectUserByUsername(ctx, username)
	if err != nil {
		return 0, customerror.WrapRepository("select user by username for verify credentials", err)
	}
	if existingUser == nil {
		return 0, customerror.ErrInvalidCredential
	}
	if existingUser.IsGuest() {
		return 0, customerror.ErrInvalidCredential
	}

	matched, err := s.passwordHasher.Matches(existingUser.Password, password)
	if err != nil {
		return 0, customerror.Wrap(customerror.ErrInternalServerError, "compare password for verify credentials", err)
	}
	if !matched {
		return 0, customerror.ErrInvalidCredential
	}
	return existingUser.ID, nil
}

func (s *UserService) EnsureAdmin(ctx context.Context, userID int64) error {
	user, err := s.userRepository.SelectUserByID(ctx, userID)
	if err != nil {
		return customerror.WrapRepository("select user by id for ensure admin", err)
	}
	if user == nil {
		return customerror.ErrUserNotFound
	}
	return s.authorizationPolicy.AdminOnly(user)
}

func (s *UserService) SuspendUser(ctx context.Context, adminID int64, targetUserUUID, reason string, duration model.SuspensionDuration) error {
	if strings.TrimSpace(reason) == "" {
		return customerror.ErrInvalidInput
	}
	entityDuration, ok := duration.ToEntity()
	if !ok {
		return customerror.ErrInvalidInput
	}
	until, ok := entityDuration.EndTime(time.Now())
	if !ok {
		return customerror.ErrInvalidInput
	}
	return s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		admin, err := tx.UserRepository().SelectUserByID(txCtx, adminID)
		if err != nil {
			return customerror.WrapRepository("select admin by id for suspend user", err)
		}
		if admin == nil {
			return customerror.ErrUserNotFound
		}
		if err := s.authorizationPolicy.AdminOnly(admin); err != nil {
			return err
		}
		target, err := tx.UserRepository().SelectUserByUUID(txCtx, targetUserUUID)
		if err != nil {
			return customerror.WrapRepository("select target user by uuid for suspend user", err)
		}
		if target == nil {
			return customerror.ErrUserNotFound
		}
		target.Suspend(strings.TrimSpace(reason), until)
		if err := tx.UserRepository().Update(txCtx, target); err != nil {
			return customerror.WrapRepository("update user suspension", err)
		}
		return nil
	})
}

func (s *UserService) GetUserSuspension(ctx context.Context, adminID int64, targetUserUUID string) (*model.UserSuspension, error) {
	var suspension *model.UserSuspension
	err := s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		admin, err := tx.UserRepository().SelectUserByID(txCtx, adminID)
		if err != nil {
			return customerror.WrapRepository("select admin by id for get user suspension", err)
		}
		if admin == nil {
			return customerror.ErrUserNotFound
		}
		if err := s.authorizationPolicy.AdminOnly(admin); err != nil {
			return err
		}
		target, err := tx.UserRepository().SelectUserByUUID(txCtx, targetUserUUID)
		if err != nil {
			return customerror.WrapRepository("select target user by uuid for get user suspension", err)
		}
		if target == nil {
			return customerror.ErrUserNotFound
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

func (s *UserService) UnsuspendUser(ctx context.Context, adminID int64, targetUserUUID string) error {
	return s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		admin, err := tx.UserRepository().SelectUserByID(txCtx, adminID)
		if err != nil {
			return customerror.WrapRepository("select admin by id for unsuspend user", err)
		}
		if admin == nil {
			return customerror.ErrUserNotFound
		}
		if err := s.authorizationPolicy.AdminOnly(admin); err != nil {
			return err
		}
		target, err := tx.UserRepository().SelectUserByUUID(txCtx, targetUserUUID)
		if err != nil {
			return customerror.WrapRepository("select target user by uuid for unsuspend user", err)
		}
		if target == nil {
			return customerror.ErrUserNotFound
		}
		target.Unsuspend()
		if err := tx.UserRepository().Update(txCtx, target); err != nil {
			return customerror.WrapRepository("clear user suspension", err)
		}
		return nil
	})
}

func normalizeUsername(username string) string {
	return strings.TrimSpace(username)
}
