package service

import (
	"fmt"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
)

var _ port.SessionUseCase = (*SessionService)(nil)

type SessionService struct {
	credentialVerifier port.CredentialVerifier
	tokenPort          port.TokenProvider
	cache              port.Cache
}

func NewSessionService(credentialVerifier port.CredentialVerifier, tokenPort port.TokenProvider, cache port.Cache) *SessionService {
	return &SessionService{
		credentialVerifier: credentialVerifier,
		tokenPort:          tokenPort,
		cache:              cache,
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

	s.cache.SetWithTTL(sessionCacheKey(userID, token), userID, s.tokenPort.TTLSeconds())
	return token, nil
}

func (s *SessionService) Logout(token string) error {
	userID, err := s.tokenPort.ValidateTokenToId(token)
	if err != nil {
		return nil
	}
	s.cache.Delete(sessionCacheKey(userID, token))
	return nil
}

func (s *SessionService) InvalidateUserSessions(userID int64) error {
	s.cache.DeleteByPrefix(sessionCachePrefix(userID))
	return nil
}

func (s *SessionService) ValidateTokenToId(token string) (int64, error) {
	userID, err := s.tokenPort.ValidateTokenToId(token)
	if err != nil {
		return 0, err
	}

	if _, exists := s.cache.Get(sessionCacheKey(userID, token)); !exists {
		return 0, customError.ErrInvalidToken
	}

	return userID, nil
}

func sessionCachePrefix(userID int64) string {
	return fmt.Sprintf("session:user:%d:", userID)
}

func sessionCacheKey(userID int64, token string) string {
	return sessionCachePrefix(userID) + token
}
