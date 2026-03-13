package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
)

func AuthWithSession(sessionUseCase port.SessionUseCase, writeError func(*gin.Context, int, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := extractToken(c.GetHeader("Authorization"))
		if err != nil {
			writeError(c, http.StatusUnauthorized, err)
			return
		}

		userID, err := sessionUseCase.ValidateTokenToId(c.Request.Context(), token)
		if err != nil {
			writeError(c, statusForAuthError(err), err)
			return
		}

		c.Set("user_id", userID)
		c.Set("auth_token", token)
		c.Next()
	}
}

func statusForAuthError(err error) int {
	switch {
	case errors.Is(err, customError.ErrMissingAuthHeader):
		return http.StatusUnauthorized
	case errors.Is(err, customError.ErrInvalidToken):
		return http.StatusUnauthorized
	case errors.Is(err, customError.ErrUnauthorized):
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}

func UserID(c *gin.Context) (int64, bool) {
	v, ok := c.Get("user_id")
	if !ok {
		return 0, false
	}
	userID, ok := v.(int64)
	return userID, ok
}

func Token(c *gin.Context) (string, bool) {
	v, ok := c.Get("auth_token")
	if !ok {
		return "", false
	}
	token, ok := v.(string)
	return token, ok
}

func extractToken(raw string) (string, error) {
	if raw == "" {
		return "", customError.ErrMissingAuthHeader
	}

	parts := strings.Fields(raw)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", customError.ErrInvalidToken
	}
	return parts[1], nil
}
