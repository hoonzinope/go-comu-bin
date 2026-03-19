package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubSessionUseCase struct {
	validate func(token string) (int64, error)
}

func (s stubSessionUseCase) Login(context.Context, string, string) (string, error) { return "", nil }
func (s stubSessionUseCase) IssueGuestToken(context.Context) (string, error)        { return "", nil }
func (s stubSessionUseCase) Logout(context.Context, string) error                  { return nil }
func (s stubSessionUseCase) InvalidateUserSessions(context.Context, int64) error   { return nil }
func (s stubSessionUseCase) ValidateTokenToId(_ context.Context, token string) (int64, error) {
	return s.validate(token)
}

func TestExtractToken(t *testing.T) {
	token, err := extractToken("Bearer abc")
	require.NoError(t, err)
	assert.Equal(t, "abc", token)

	_, err = extractToken("invalid")
	require.Error(t, err)
}

func TestAuthWithSessionInjectsContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthWithSession(stubSessionUseCase{
		validate: func(token string) (int64, error) {
			assert.Equal(t, "abc", token)
			return 7, nil
		},
	}, func(c *gin.Context, status int, err error) {
		c.AbortWithStatus(status)
	}))
	r.GET("/", func(c *gin.Context) {
		userID, ok := UserID(c)
		require.True(t, ok)
		token, ok := Token(c)
		require.True(t, ok)
		c.JSON(http.StatusOK, gin.H{"user_id": userID, "token": token})
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer abc")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.JSONEq(t, `{"token":"abc","user_id":7}`, rr.Body.String())
}

func TestAuthWithSessionRejectsInvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthWithSession(stubSessionUseCase{
		validate: func(token string) (int64, error) {
			return 0, customerror.ErrInvalidToken
		},
	}, func(c *gin.Context, status int, err error) {
		c.AbortWithStatus(status)
	}))
	r.GET("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer abc")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAuthWithSessionReturnsInternalServerErrorOnRepositoryFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthWithSession(stubSessionUseCase{
		validate: func(token string) (int64, error) {
			return 0, customerror.WrapRepository("lookup session", errors.New("cache down"))
		},
	}, func(c *gin.Context, status int, err error) {
		c.AbortWithStatus(status)
	}))
	r.GET("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer abc")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}
