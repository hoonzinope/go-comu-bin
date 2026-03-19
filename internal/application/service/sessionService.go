package service

import (
	"context"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
)

var _ port.SessionUseCase = (*SessionService)(nil)

type SessionService struct {
	credentialVerifier port.CredentialVerifier
	guestIssuer        port.GuestAccountIssuer
	userRepository     port.UserRepository
	tokenPort          port.TokenProvider
	sessionRepository  port.SessionRepository
}

func NewSessionService(credentialVerifier port.CredentialVerifier, guestIssuer port.GuestAccountIssuer, userRepository port.UserRepository, tokenPort port.TokenProvider, sessionRepository port.SessionRepository) *SessionService {
	return &SessionService{
		credentialVerifier: credentialVerifier,
		guestIssuer:        guestIssuer,
		userRepository:     userRepository,
		tokenPort:          tokenPort,
		sessionRepository:  sessionRepository,
	}
}

func (s *SessionService) Login(ctx context.Context, username, password string) (string, error) {
	userID, err := s.credentialVerifier.VerifyCredentials(ctx, username, password)
	if err != nil {
		return "", err
	}

	token, err := s.tokenPort.IdToToken(userID)
	if err != nil {
		return "", customerror.WrapToken("issue login token", err)
	}

	if err := s.sessionRepository.Save(ctx, userID, token, s.tokenPort.TTLSeconds()); err != nil {
		return "", customerror.WrapRepository("save session", err)
	}
	return token, nil
}

func (s *SessionService) IssueGuestToken(ctx context.Context) (string, error) {
	if s.guestIssuer == nil {
		return "", customerror.ErrInternalServerError
	}
	userID, err := s.guestIssuer.IssueGuestAccount(ctx)
	if err != nil {
		return "", err
	}

	token, err := s.tokenPort.IdToToken(userID)
	if err != nil {
		return "", customerror.WrapToken("issue guest token", err)
	}

	if err := s.sessionRepository.Save(ctx, userID, token, s.tokenPort.TTLSeconds()); err != nil {
		s.expireGuestBestEffort(ctx, userID)
		return "", customerror.WrapRepository("save guest session", err)
	}
	if err := s.markGuestActive(ctx, userID); err != nil {
		return "", err
	}
	return token, nil
}

func (s *SessionService) RotateToken(ctx context.Context, userID int64, currentToken string) (string, error) {
	if err := s.sessionRepository.Delete(ctx, userID, currentToken); err != nil {
		return "", customerror.WrapRepository("delete current session for rotate token", err)
	}

	token, err := s.tokenPort.IdToToken(userID)
	if err != nil {
		return "", customerror.WrapToken("issue rotated token", err)
	}
	if err := s.sessionRepository.Save(ctx, userID, token, s.tokenPort.TTLSeconds()); err != nil {
		return "", customerror.WrapRepository("save rotated session", err)
	}
	return token, nil
}

func (s *SessionService) Logout(ctx context.Context, token string) error {
	userID, err := s.tokenPort.ValidateTokenToId(token)
	if err != nil {
		return nil
	}
	if err := s.sessionRepository.Delete(ctx, userID, token); err != nil {
		return customerror.WrapRepository("delete session", err)
	}
	return nil
}

func (s *SessionService) InvalidateUserSessions(ctx context.Context, userID int64) error {
	if err := s.sessionRepository.DeleteByUser(ctx, userID); err != nil {
		return customerror.WrapRepository("delete user sessions", err)
	}
	return nil
}

func (s *SessionService) ValidateTokenToId(ctx context.Context, token string) (int64, error) {
	userID, err := s.tokenPort.ValidateTokenToId(token)
	if err != nil {
		return 0, err
	}

	exists, err := s.sessionRepository.Exists(ctx, userID, token)
	if err != nil {
		return 0, customerror.WrapRepository("lookup session", err)
	}
	if !exists {
		return 0, customerror.ErrInvalidToken
	}
	user, err := s.userRepository.SelectUserByID(ctx, userID)
	if err != nil {
		return 0, customerror.WrapRepository("select user by id for validate token", err)
	}
	if user == nil {
		return 0, customerror.ErrInvalidToken
	}
	if user.IsGuest() && !user.IsActiveGuest() {
		return 0, customerror.ErrInvalidToken
	}

	return userID, nil
}

func (s *SessionService) markGuestActive(ctx context.Context, userID int64) error {
	user, err := s.userRepository.SelectUserByID(ctx, userID)
	if err != nil {
		return customerror.WrapRepository("select user by id for activate guest", err)
	}
	if user == nil || !user.IsGuest() {
		return nil
	}
	user.MarkGuestActive()
	if err := s.userRepository.Update(ctx, user); err != nil {
		return customerror.WrapRepository("update guest active state", err)
	}
	return nil
}

func (s *SessionService) expireGuestBestEffort(ctx context.Context, userID int64) {
	user, err := s.userRepository.SelectUserByID(ctx, userID)
	if err != nil || user == nil || !user.IsGuest() {
		return
	}
	user.MarkGuestExpired()
	_ = s.userRepository.Update(ctx, user)
}
