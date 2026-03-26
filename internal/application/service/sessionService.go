package service

import (
	"log/slog"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	sessionsvc "github.com/hoonzinope/go-comu-bin/internal/application/service/session"
)

func NewSessionService(credentialVerifier port.CredentialVerifier, guestIssuer port.GuestAccountIssuer, userRepository port.UserRepository, tokenPort port.TokenProvider, sessionRepository port.SessionRepository, logger ...*slog.Logger) *sessionsvc.SessionService {
	return sessionsvc.NewSessionService(credentialVerifier, guestIssuer, userRepository, tokenPort, sessionRepository, logger...)
}
