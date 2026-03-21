package account

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	svccommon "github.com/hoonzinope/go-comu-bin/internal/application/service/common"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.AccountUseCase = (*AccountService)(nil)

type Service = AccountService

type AccountService struct {
	userUseCase    port.UserUseCase
	sessionUseCase port.SessionUseCase
	userRepository port.UserRepository
	unitOfWork     port.UnitOfWork
	passwordHasher port.PasswordHasher
	tokenProvider  port.TokenProvider
	sessionRepo    port.SessionRepository
	logger         *slog.Logger
}

func NewAccountService(userUseCase port.UserUseCase, sessionUseCase port.SessionUseCase, logger ...*slog.Logger) *AccountService {
	return &AccountService{
		userUseCase:    userUseCase,
		sessionUseCase: sessionUseCase,
		logger:         svccommon.ResolveLogger(logger),
	}
}

func NewService(userUseCase port.UserUseCase, sessionUseCase port.SessionUseCase, logger ...*slog.Logger) *Service {
	return NewAccountService(userUseCase, sessionUseCase, logger...)
}

func NewAccountServiceWithGuestUpgrade(
	userUseCase port.UserUseCase,
	sessionUseCase port.SessionUseCase,
	userRepository port.UserRepository,
	unitOfWork port.UnitOfWork,
	passwordHasher port.PasswordHasher,
	tokenProvider port.TokenProvider,
	sessionRepository port.SessionRepository,
	logger ...*slog.Logger,
) *AccountService {
	svc := NewAccountService(userUseCase, sessionUseCase, logger...)
	svc.userRepository = userRepository
	svc.unitOfWork = unitOfWork
	svc.passwordHasher = passwordHasher
	svc.tokenProvider = tokenProvider
	svc.sessionRepo = sessionRepository
	return svc
}

func NewServiceWithGuestUpgrade(
	userUseCase port.UserUseCase,
	sessionUseCase port.SessionUseCase,
	userRepository port.UserRepository,
	unitOfWork port.UnitOfWork,
	passwordHasher port.PasswordHasher,
	tokenProvider port.TokenProvider,
	sessionRepository port.SessionRepository,
	logger ...*slog.Logger,
) *Service {
	return NewAccountServiceWithGuestUpgrade(
		userUseCase,
		sessionUseCase,
		userRepository,
		unitOfWork,
		passwordHasher,
		tokenProvider,
		sessionRepository,
		logger...,
	)
}

func (s *AccountService) DeleteMyAccount(ctx context.Context, userID int64, password string) error {
	if err := s.userUseCase.DeleteMe(ctx, userID, password); err != nil {
		return err
	}
	if err := s.sessionUseCase.InvalidateUserSessions(ctx, userID); err != nil {
		s.logger.Warn("failed to invalidate deleted user sessions", "user_id", userID, "error", err)
	}
	return nil
}

func (s *AccountService) UpgradeGuestAccount(ctx context.Context, userID int64, currentToken, username, email, password string) (string, error) {
	if s.userRepository == nil || s.unitOfWork == nil || s.passwordHasher == nil || s.tokenProvider == nil || s.sessionRepo == nil {
		return "", customerror.ErrInternalServerError
	}
	username = strings.TrimSpace(username)
	email = strings.TrimSpace(email)
	currentToken = strings.TrimSpace(currentToken)
	if username == "" || email == "" || strings.TrimSpace(password) == "" || currentToken == "" {
		return "", customerror.ErrInvalidInput
	}
	hashedPassword, err := s.passwordHasher.Hash(password)
	if err != nil {
		return "", customerror.Wrap(customerror.ErrInternalServerError, "hash password for guest upgrade", err)
	}

	var newToken string
	err = s.sessionRepo.WithUserLock(ctx, userID, func(scope port.SessionRepositoryScope) error {
		exists, err := scope.Exists(ctx, userID, currentToken)
		if err != nil {
			return customerror.WrapRepository("lookup current session for guest upgrade", err)
		}
		if !exists {
			return customerror.ErrInvalidToken
		}

		candidateToken, err := s.tokenProvider.IdToToken(userID)
		if err != nil {
			return customerror.WrapToken("issue guest upgrade token", err)
		}
		if err := scope.Save(ctx, userID, candidateToken, s.tokenProvider.TTLSeconds()); err != nil {
			return customerror.WrapRepository("save upgraded session", err)
		}

		originalUser, err := s.applyGuestUpgrade(ctx, userID, username, email, hashedPassword)
		if err != nil {
			_ = scope.Delete(ctx, userID, candidateToken)
			return err
		}

		if err := scope.Delete(ctx, userID, currentToken); err != nil {
			restoreErr := s.restoreUserState(ctx, originalUser)
			_ = scope.Delete(ctx, userID, candidateToken)
			if restoreErr != nil {
				return errors.Join(
					customerror.WrapRepository("delete current session for guest upgrade", err),
					customerror.WrapRepository("restore guest after session rollback", restoreErr),
				)
			}
			return customerror.WrapRepository("delete current session for guest upgrade", err)
		}

		newToken = candidateToken
		return nil
	})
	if err != nil {
		return "", err
	}
	return newToken, nil
}

func (s *AccountService) applyGuestUpgrade(ctx context.Context, userID int64, username, email, hashedPassword string) (*entity.User, error) {
	var original *entity.User
	err := s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
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
		originalCopy := *existingUser
		original = &originalCopy

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
	if err != nil {
		return nil, err
	}
	return original, nil
}

func (s *AccountService) restoreUserState(ctx context.Context, user *entity.User) error {
	if user == nil {
		return nil
	}
	return s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		current, err := tx.UserRepository().SelectUserByIDIncludingDeleted(txCtx, user.ID)
		if err != nil {
			return customerror.WrapRepository("select user by id for guest rollback", err)
		}
		if current == nil {
			return customerror.ErrUserNotFound
		}
		restoreCopy := *user
		if err := tx.UserRepository().Update(txCtx, &restoreCopy); err != nil {
			return customerror.WrapRepository("restore guest after failed upgrade", err)
		}
		return nil
	})
}
