package service

import (
	"github.com/hoonzinope/go-comu-bin/internal/application"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
)

var _ application.SessionUseCase = (*SessionService)(nil)

type SessionService struct {
	userUseCase application.UserUseCase
	tokenPort   application.TokenProvider
	cache       application.Cache
}

func NewSessionService(userUseCase application.UserUseCase, tokenPort application.TokenProvider, cache application.Cache) *SessionService {
	return &SessionService{
		userUseCase: userUseCase,
		tokenPort:   tokenPort,
		cache:       cache,
	}
}

func (s *SessionService) Login(username, password string) (string, error) {
	userID, err := s.userUseCase.Login(username, password)
	if err != nil {
		return "", err
	}

	token, err := s.tokenPort.IdToToken(userID)
	if err != nil {
		return "", customError.ErrInternalServerError
	}

	s.cache.Set(token, userID)
	return token, nil
}

func (s *SessionService) Logout(token string) error {
	s.cache.Delete(token)
	return nil
}

func (s *SessionService) ValidateTokenToId(token string) (int64, error) {
	userID, err := s.tokenPort.ValidateTokenToId(token)
	if err != nil {
		return 0, err
	}

	if _, exists := s.cache.Get(token); !exists {
		return 0, customError.ErrInvalidToken
	}

	return userID, nil
}
