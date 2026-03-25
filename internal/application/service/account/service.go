package account

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/mail"
	"strings"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	svccommon "github.com/hoonzinope/go-comu-bin/internal/application/service/common"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.AccountUseCase = (*AccountService)(nil)

type Service = AccountService

type AccountService struct {
	userUseCase          port.UserUseCase
	sessionUseCase       port.SessionUseCase
	userRepository       port.UserRepository
	unitOfWork           port.UnitOfWork
	passwordHasher       port.PasswordHasher
	tokenProvider        port.TokenProvider
	sessionRepo          port.SessionRepository
	verificationTokens   port.EmailVerificationTokenRepository
	verificationIssuer   port.EmailVerificationTokenIssuer
	verificationMailer   port.EmailVerificationMailSender
	verificationTokenTTL time.Duration
	resetTokens          port.PasswordResetTokenRepository
	resetIssuer          port.PasswordResetTokenIssuer
	resetMailer          port.PasswordResetMailSender
	resetTokenTTL        time.Duration
	logger               *slog.Logger
}

const defaultPasswordResetTokenTTL = 30 * time.Minute
const defaultEmailVerificationTokenTTL = 30 * time.Minute

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
	verificationTokens port.EmailVerificationTokenRepository,
	verificationIssuer port.EmailVerificationTokenIssuer,
	verificationMailer port.EmailVerificationMailSender,
	verificationTokenTTL time.Duration,
	resetTokens port.PasswordResetTokenRepository,
	resetIssuer port.PasswordResetTokenIssuer,
	resetMailer port.PasswordResetMailSender,
	resetTokenTTL time.Duration,
	logger ...*slog.Logger,
) *AccountService {
	svc := NewAccountService(userUseCase, sessionUseCase, logger...)
	svc.userRepository = userRepository
	svc.unitOfWork = unitOfWork
	svc.passwordHasher = passwordHasher
	svc.tokenProvider = tokenProvider
	svc.sessionRepo = sessionRepository
	svc.verificationTokens = verificationTokens
	svc.verificationIssuer = verificationIssuer
	svc.verificationMailer = verificationMailer
	svc.verificationTokenTTL = verificationTokenTTL
	svc.resetTokens = resetTokens
	svc.resetIssuer = resetIssuer
	svc.resetMailer = resetMailer
	svc.resetTokenTTL = resetTokenTTL
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
	verificationTokens port.EmailVerificationTokenRepository,
	verificationIssuer port.EmailVerificationTokenIssuer,
	verificationMailer port.EmailVerificationMailSender,
	verificationTokenTTL time.Duration,
	resetTokens port.PasswordResetTokenRepository,
	resetIssuer port.PasswordResetTokenIssuer,
	resetMailer port.PasswordResetMailSender,
	resetTokenTTL time.Duration,
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
		verificationTokens,
		verificationIssuer,
		verificationMailer,
		verificationTokenTTL,
		resetTokens,
		resetIssuer,
		resetMailer,
		resetTokenTTL,
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
		s.logGuestUpgradeAttempt(userID, "invalid_input")
		return "", customerror.ErrInvalidInput
	}
	hashedPassword, err := s.passwordHasher.Hash(password)
	if err != nil {
		s.logGuestUpgradeAttempt(userID, "failed")
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
		if errors.Is(err, customerror.ErrInvalidToken) {
			s.logGuestUpgradeAttempt(userID, "invalid_token")
		} else if errors.Is(err, customerror.ErrInvalidInput) {
			s.logGuestUpgradeAttempt(userID, "invalid_input")
		} else {
			s.logGuestUpgradeAttempt(userID, "failed")
		}
		return "", err
	}
	s.logGuestUpgradeAttempt(userID, "succeeded")
	return newToken, nil
}

func (s *AccountService) RequestEmailVerification(ctx context.Context, userID int64) error {
	if s.unitOfWork == nil || s.verificationTokens == nil || s.verificationIssuer == nil || s.verificationMailer == nil {
		return customerror.ErrInternalServerError
	}
	if userID <= 0 {
		s.logEmailVerificationRequest(userID, "ignored_unknown_or_ineligible")
		return nil
	}
	outcome := "ignored_unknown_or_ineligible"
	err := s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		user, err := tx.UserRepository().SelectUserByID(txCtx, userID)
		if err != nil {
			return customerror.WrapRepository("select user by id for email verification request", err)
		}
		if user == nil {
			return nil
		}
		if user.IsGuest() || user.IsDeleted() || user.Email == "" || user.IsEmailVerified() {
			return nil
		}
		if err := s.issueAndSendEmailVerification(txCtx, tx, user); err != nil {
			return err
		}
		outcome = "issued"
		return nil
	})
	if err != nil {
		if strings.Contains(err.Error(), "send email verification mail") {
			s.logEmailVerificationRequest(userID, "mail_failed")
		}
		return err
	}
	s.logEmailVerificationRequest(userID, outcome)
	return nil
}

func (s *AccountService) ConfirmEmailVerification(ctx context.Context, token string) error {
	if s.unitOfWork == nil || s.verificationTokens == nil {
		return customerror.ErrInternalServerError
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return customerror.ErrInvalidInput
	}
	tokenHash := hashEmailVerificationToken(token)
	var confirmedUserID int64
	err := s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		verificationToken, err := tx.EmailVerificationTokenRepository().SelectByTokenHash(txCtx, tokenHash)
		if err != nil {
			return customerror.WrapRepository("select email verification token for confirm", err)
		}
		if verificationToken == nil || !verificationToken.IsUsable(time.Now()) {
			return customerror.ErrInvalidToken
		}
		user, err := tx.UserRepository().SelectUserByID(txCtx, verificationToken.UserID)
		if err != nil {
			return customerror.WrapRepository("select user by id for email verification confirm", err)
		}
		if user == nil || user.IsGuest() || user.IsDeleted() || user.Email == "" {
			return customerror.ErrInvalidToken
		}
		now := time.Now()
		if !user.IsEmailVerified() {
			user.MarkEmailVerified(now)
			if err := tx.UserRepository().Update(txCtx, user); err != nil {
				return customerror.WrapRepository("update user for email verification confirm", err)
			}
		}
		if err := tx.EmailVerificationTokenRepository().InvalidateByUser(txCtx, user.ID); err != nil {
			return customerror.WrapRepository("invalidate email verification tokens", err)
		}
		confirmedUserID = user.ID
		return nil
	})
	if err != nil {
		if errors.Is(err, customerror.ErrInvalidToken) {
			s.logEmailVerificationConfirm(confirmedUserID, "invalid_token")
		}
		return err
	}
	s.logEmailVerificationConfirm(confirmedUserID, "confirmed")
	return nil
}

func (s *AccountService) RequestPasswordReset(ctx context.Context, email string) error {
	if s.unitOfWork == nil || s.resetTokens == nil || s.resetIssuer == nil || s.resetMailer == nil {
		return customerror.ErrInternalServerError
	}
	email = normalizeAccountEmail(email)
	if email == "" {
		return customerror.ErrInvalidInput
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return customerror.ErrInvalidInput
	}
	tokenTTL := s.resetTokenTTL
	if tokenTTL <= 0 {
		tokenTTL = defaultPasswordResetTokenTTL
	}

	var (
		userID    int64
		rawToken  string
		expiresAt time.Time
	)
	err := s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		user, err := tx.UserRepository().SelectUserByEmail(txCtx, email)
		if err != nil {
			return customerror.WrapRepository("select user by email for password reset request", err)
		}
		if user == nil || user.IsGuest() || user.IsDeleted() {
			return nil
		}
		rawToken, err = s.resetIssuer.Issue()
		if err != nil {
			return customerror.Wrap(customerror.ErrInternalServerError, "issue password reset token", err)
		}
		expiresAt = time.Now().Add(tokenTTL)
		userID = user.ID
		if err := tx.PasswordResetTokenRepository().InvalidateByUser(txCtx, user.ID); err != nil {
			return customerror.WrapRepository("invalidate previous password reset tokens", err)
		}
		if err := tx.PasswordResetTokenRepository().Save(txCtx, entity.NewPasswordResetToken(user.ID, hashResetToken(rawToken), expiresAt)); err != nil {
			return customerror.WrapRepository("save password reset token", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if rawToken == "" {
		s.logPasswordResetRequest(email, "ignored_unknown_or_ineligible")
		return err
	}
	if err := s.resetMailer.SendPasswordReset(ctx, email, rawToken, expiresAt); err != nil {
		s.logPasswordResetRequest(email, "mail_failed")
		rollbackErr := s.invalidatePasswordResetTokens(ctx, userID)
		if rollbackErr != nil {
			return errors.Join(
				customerror.Wrap(customerror.ErrInternalServerError, "send password reset mail", err),
				customerror.WrapRepository("rollback password reset token after mail failure", rollbackErr),
			)
		}
		return customerror.Wrap(customerror.ErrInternalServerError, "send password reset mail", err)
	}
	s.logPasswordResetRequest(email, "issued")
	return nil
}

func (s *AccountService) ConfirmPasswordReset(ctx context.Context, token, newPassword string) error {
	if s.unitOfWork == nil || s.passwordHasher == nil || s.sessionRepo == nil || s.resetTokens == nil {
		return customerror.ErrInternalServerError
	}
	token = strings.TrimSpace(token)
	if token == "" || strings.TrimSpace(newPassword) == "" {
		return customerror.ErrInvalidInput
	}
	tokenHash := hashResetToken(token)
	existingReset, err := s.resetTokens.SelectByTokenHash(ctx, tokenHash)
	if err != nil {
		return customerror.WrapRepository("select password reset token before confirm", err)
	}
	if existingReset == nil {
		s.logPasswordResetConfirm(0, "invalid_token")
		return customerror.ErrInvalidToken
	}
	userID := existingReset.UserID
	hashedPassword, err := s.passwordHasher.Hash(newPassword)
	if err != nil {
		return customerror.Wrap(customerror.ErrInternalServerError, "hash password for password reset confirm", err)
	}

	err = s.sessionRepo.WithUserLock(ctx, userID, func(scope port.SessionRepositoryScope) error {
		err := s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
			txCtx := tx.Context()
			resetToken, err := tx.PasswordResetTokenRepository().SelectByTokenHash(txCtx, tokenHash)
			if err != nil {
				return customerror.WrapRepository("select password reset token before session invalidation", err)
			}
			if resetToken == nil || !resetToken.IsUsable(time.Now()) {
				return customerror.ErrInvalidToken
			}
			user, err := tx.UserRepository().SelectUserByID(txCtx, resetToken.UserID)
			if err != nil {
				return customerror.WrapRepository("select user by id before session invalidation", err)
			}
			if user == nil || user.IsGuest() || user.IsDeleted() {
				return customerror.ErrInvalidToken
			}
			return nil
		})
		if err != nil {
			return err
		}
		if err := scope.DeleteByUser(ctx, userID); err != nil {
			return customerror.WrapRepository("delete user sessions for password reset confirm", err)
		}
		return s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
			txCtx := tx.Context()
			resetToken, err := tx.PasswordResetTokenRepository().SelectByTokenHash(txCtx, tokenHash)
			if err != nil {
				return customerror.WrapRepository("select password reset token for confirm", err)
			}
			if resetToken == nil || !resetToken.IsUsable(time.Now()) {
				return customerror.ErrInvalidToken
			}
			user, err := tx.UserRepository().SelectUserByID(txCtx, resetToken.UserID)
			if err != nil {
				return customerror.WrapRepository("select user by id for password reset confirm", err)
			}
			if user == nil || user.IsGuest() || user.IsDeleted() {
				return customerror.ErrInvalidToken
			}

			user.Password = hashedPassword
			user.UpdatedAt = time.Now()
			if err := tx.UserRepository().Update(txCtx, user); err != nil {
				return customerror.WrapRepository("update user password for password reset confirm", err)
			}
			resetToken.Consume(time.Now())
			if err := tx.PasswordResetTokenRepository().Update(txCtx, resetToken); err != nil {
				return customerror.WrapRepository("consume password reset token", err)
			}
			return nil
		})
	})
	if err != nil {
		if errors.Is(err, customerror.ErrInvalidToken) {
			s.logPasswordResetConfirm(userID, "invalid_token")
		} else if errors.Is(err, customerror.ErrRepositoryFailure) {
			s.logPasswordResetConfirm(userID, "session_invalidation_failed")
		}
		return err
	}
	s.logPasswordResetConfirm(userID, "confirmed")
	return err
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
		if err := s.issueAndSendEmailVerification(txCtx, tx, &upgradedUser); err != nil {
			return err
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

func (s *AccountService) restorePasswordResetState(ctx context.Context, user *entity.User, token *entity.PasswordResetToken) error {
	return s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		if user != nil {
			current, err := tx.UserRepository().SelectUserByIDIncludingDeleted(txCtx, user.ID)
			if err != nil {
				return customerror.WrapRepository("select user by id for password reset rollback", err)
			}
			if current == nil {
				return customerror.ErrUserNotFound
			}
			restoreUser := *user
			if err := tx.UserRepository().Update(txCtx, &restoreUser); err != nil {
				return customerror.WrapRepository("restore user after failed password reset", err)
			}
		}
		if token != nil {
			restoreToken := *token
			if err := tx.PasswordResetTokenRepository().Update(txCtx, &restoreToken); err != nil {
				return customerror.WrapRepository("restore password reset token after failed password reset", err)
			}
		}
		return nil
	})
}

func (s *AccountService) invalidatePasswordResetTokens(ctx context.Context, userID int64) error {
	return s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		if err := tx.PasswordResetTokenRepository().InvalidateByUser(tx.Context(), userID); err != nil {
			return customerror.WrapRepository("invalidate password reset tokens", err)
		}
		return nil
	})
}

func (s *AccountService) issueAndSendEmailVerification(ctx context.Context, tx port.TxScope, user *entity.User) error {
	if tx == nil || user == nil || s.verificationTokens == nil || s.verificationIssuer == nil || s.verificationMailer == nil {
		return nil
	}
	if user.Email == "" || user.IsEmailVerified() {
		return nil
	}
	tokenTTL := s.verificationTokenTTL
	if tokenTTL <= 0 {
		tokenTTL = defaultEmailVerificationTokenTTL
	}
	rawToken, err := s.verificationIssuer.Issue()
	if err != nil {
		return customerror.Wrap(customerror.ErrInternalServerError, "issue email verification token", err)
	}
	expiresAt := time.Now().Add(tokenTTL)
	if err := tx.EmailVerificationTokenRepository().InvalidateByUser(ctx, user.ID); err != nil {
		return customerror.WrapRepository("invalidate previous email verification tokens", err)
	}
	if err := tx.EmailVerificationTokenRepository().Save(ctx, entity.NewEmailVerificationToken(user.ID, hashEmailVerificationToken(rawToken), expiresAt)); err != nil {
		return customerror.WrapRepository("save email verification token", err)
	}
	if err := s.verificationMailer.SendEmailVerification(ctx, user.Email, rawToken, expiresAt); err != nil {
		return customerror.Wrap(customerror.ErrInternalServerError, "send email verification mail", err)
	}
	return nil
}

func hashResetToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func hashEmailVerificationToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *AccountService) logEmailVerificationRequest(userID int64, outcome string) {
	s.logger.Info(
		"email verification request audit",
		"event", "email_verification_request",
		"user_id", userID,
		"outcome", outcome,
	)
}

func (s *AccountService) logEmailVerificationConfirm(userID int64, outcome string) {
	s.logger.Info(
		"email verification confirm audit",
		"event", "email_verification_confirm",
		"user_id", userID,
		"outcome", outcome,
	)
}

func normalizeAccountEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (s *AccountService) logPasswordResetRequest(email, outcome string) {
	if s == nil || s.logger == nil {
		return
	}
	s.logger.Info(
		"password reset request",
		"event", "password_reset_request",
		"outcome", outcome,
		"email_sha256", hashResetToken(normalizeAccountEmail(email)),
	)
}

func (s *AccountService) logPasswordResetConfirm(userID int64, outcome string) {
	if s == nil || s.logger == nil {
		return
	}
	attrs := []any{
		"event", "password_reset_confirm",
		"outcome", outcome,
	}
	if userID > 0 {
		attrs = append(attrs, "user_id", userID)
	}
	s.logger.Info("password reset confirm", attrs...)
}

func (s *AccountService) logGuestUpgradeAttempt(userID int64, outcome string) {
	if s == nil || s.logger == nil {
		return
	}
	s.logger.Info(
		"guest upgrade audit",
		"event", "guest_upgrade_attempt",
		"user_id", userID,
		"outcome", outcome,
	)
}
