package service

import (
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
)

var _ port.SessionUseCase = (*SessionService)(nil)

type SessionService struct {
	credentialVerifier port.CredentialVerifier
	userRepository     port.UserRepository
	tokenPort          port.TokenProvider
	sessionRepository  port.SessionRepository
}

func NewSessionService(credentialVerifier port.CredentialVerifier, userRepository port.UserRepository, tokenPort port.TokenProvider, sessionRepository port.SessionRepository) *SessionService {
	return &SessionService{
		credentialVerifier: credentialVerifier,
		userRepository:     userRepository,
		tokenPort:          tokenPort,
		sessionRepository:  sessionRepository,
	}
}

func (s *SessionService) Login(username, password string) (string, error) {
	userID, err := s.credentialVerifier.VerifyCredentials(username, password)
	if err != nil {
		return "", err
	}

	token, err := s.tokenPort.IdToToken(userID)
	if err != nil {
		return "", customError.WrapToken("issue login token", err)
	}

	if err := s.sessionRepository.Save(userID, token, s.tokenPort.TTLSeconds()); err != nil {
		return "", customError.WrapRepository("save session", err)
	}
	return token, nil
}

func (s *SessionService) Logout(token string) error {
	userID, err := s.tokenPort.ValidateTokenToId(token)
	if err != nil {
		return nil
	}
	if err := s.sessionRepository.Delete(userID, token); err != nil {
		return customError.WrapRepository("delete session", err)
	}
	return nil
}

func (s *SessionService) InvalidateUserSessions(userID int64) error {
	if err := s.sessionRepository.DeleteByUser(userID); err != nil {
		return customError.WrapRepository("delete user sessions", err)
	}
	return nil
}

func (s *SessionService) ValidateTokenToId(token string) (int64, error) {
	userID, err := s.tokenPort.ValidateTokenToId(token)
	if err != nil {
		return 0, err
	}

	exists, err := s.sessionRepository.Exists(userID, token)
	if err != nil {
		return 0, customError.WrapRepository("lookup session", err)
	}
	if !exists {
		return 0, customError.ErrInvalidToken
	}
	user, err := s.userRepository.SelectUserByID(userID)
	if err != nil {
		return 0, customError.WrapRepository("select user by id for validate token", err)
	}
	if user == nil {
		return 0, customError.ErrInvalidToken
	}

	return userID, nil
}
