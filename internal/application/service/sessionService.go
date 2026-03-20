package service

import (
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	sessionsvc "github.com/hoonzinope/go-comu-bin/internal/application/service/session"
)

type SessionService = sessionsvc.Service

func NewSessionService(credentialVerifier port.CredentialVerifier, guestIssuer port.GuestAccountIssuer, userRepository port.UserRepository, tokenPort port.TokenProvider, sessionRepository port.SessionRepository) *SessionService {
	return sessionsvc.NewService(credentialVerifier, guestIssuer, userRepository, tokenPort, sessionRepository)
}
