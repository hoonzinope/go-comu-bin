package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/hoonzinope/go-comu-bin/internal/application"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
)

type AuthMiddleware struct {
	AuthUseCase application.AuthUseCase
}

type contextKey string

const userIDContextKey contextKey = "user_id"

func NewAuthMiddleware(authUseCase application.AuthUseCase) *AuthMiddleware {
	return &AuthMiddleware{
		AuthUseCase: authUseCase,
	}
}

func (m *AuthMiddleware) Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := extractToken(r.Header.Get("Authorization"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		userID, err := m.AuthUseCase.ValidateTokenToId(token)
		if err != nil {
			http.Error(w, customError.ErrInvalidToken.Error(), http.StatusUnauthorized)
			return
		}

		// Set user ID in context for downstream handlers
		ctx := r.Context()
		ctx = context.WithValue(ctx, userIDContextKey, userID)
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}

func (m *AuthMiddleware) AuthByMethods(next http.Handler, methods ...string) http.Handler {
	protected := make(map[string]struct{}, len(methods))
	for _, method := range methods {
		protected[method] = struct{}{}
	}
	authenticated := m.Auth(next)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := protected[r.Method]; !ok {
			next.ServeHTTP(w, r)
			return
		}
		authenticated.ServeHTTP(w, r)
	})
}

func extractToken(raw string) (string, error) {
	if raw == "" {
		return "", customError.ErrMissingAuthHeader
	}

	parts := strings.Fields(raw)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		if parts[1] == "" {
			return "", customError.ErrInvalidToken
		}
		return parts[1], nil
	}

	// Backward compatibility: allow raw token string.
	return raw, nil
}

func UserIDFromContext(ctx context.Context) (int64, bool) {
	userID, ok := ctx.Value(userIDContextKey).(int64)
	return userID, ok
}
