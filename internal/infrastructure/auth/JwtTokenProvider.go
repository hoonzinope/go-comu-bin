package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
)

type JwtTokenProvider struct {
	secretKey string
	ttl       time.Duration
}

type tokenClaims struct {
	UserID int64 `json:"user_id"`
	jwt.RegisteredClaims
}

func NewJwtTokenProvider(secretKey string) *JwtTokenProvider {
	return &JwtTokenProvider{
		secretKey: secretKey,
		ttl:       24 * time.Hour,
	}
}

func (p *JwtTokenProvider) IdToToken(userID int64) (string, error) {
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, tokenClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(p.ttl)),
		},
	})
	signed, err := token.SignedString([]byte(p.secretKey))
	if err != nil {
		return "", customerror.WrapToken("sign jwt", err)
	}
	return signed, nil
}

func (p *JwtTokenProvider) TTLSeconds() int {
	return int(p.ttl / time.Second)
}

func (p *JwtTokenProvider) ValidateTokenToId(token string) (int64, error) {
	claims := &tokenClaims{}
	parsedToken, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(p.secretKey), nil
	})
	if err != nil {
		return 0, customerror.Wrap(customerror.ErrInvalidToken, "parse jwt", err)
	}

	if parsedToken.Valid && claims.UserID > 0 {
		return claims.UserID, nil
	}

	return 0, customerror.Wrap(customerror.ErrInvalidToken, "decode jwt claims", errors.New("invalid token claims"))
}
