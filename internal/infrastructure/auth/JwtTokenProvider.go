package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
)

type JwtTokenProvider struct {
	secretKey string
}

func NewJwtTokenProvider(secretKey string) *JwtTokenProvider {
	return &JwtTokenProvider{secretKey: secretKey}
}

func (p *JwtTokenProvider) IdToToken(userID int64) (string, error) {
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"iat":     now.Unix(),
		"exp":     now.Add(24 * time.Hour).Unix(),
	})
	return token.SignedString([]byte(p.secretKey))
}

func (p *JwtTokenProvider) ValidateTokenToId(token string) (int64, error) {
	parsedToken, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(p.secretKey), nil
	})
	if err != nil {
		return 0, fmt.Errorf("%w: %v", customError.ErrInvalidToken, err)
	}

	if claims, ok := parsedToken.Claims.(jwt.MapClaims); ok && parsedToken.Valid {
		if userID, ok := claims["user_id"].(float64); ok {
			return int64(userID), nil
		}
	}

	return 0, fmt.Errorf("%w: %v", customError.ErrInvalidToken, errors.New("invalid token claims"))
}
