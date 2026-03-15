package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubAdminAuthorizer struct {
	ensureAdmin func(ctx context.Context, userID int64) error
}

func (s stubAdminAuthorizer) EnsureAdmin(ctx context.Context, userID int64) error {
	return s.ensureAdmin(ctx, userID)
}

func TestAdminOnly_RejectsNonAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_id", int64(2))
		c.Next()
	})
	r.Use(AdminOnly(stubAdminAuthorizer{
		ensureAdmin: func(ctx context.Context, userID int64) error {
			assert.Equal(t, int64(2), userID)
			return customerror.ErrForbidden
		},
	}, func(c *gin.Context, status int, err error) {
		c.AbortWithStatusJSON(status, gin.H{"error": customerror.Public(err).Error()})
	}))
	r.GET("/admin", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestAdminOnly_AllowsAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_id", int64(1))
		c.Next()
	})
	r.Use(AdminOnly(stubAdminAuthorizer{
		ensureAdmin: func(ctx context.Context, userID int64) error {
			assert.Equal(t, int64(1), userID)
			return nil
		},
	}, func(c *gin.Context, status int, err error) {
		c.AbortWithStatus(status)
	}))
	r.GET("/admin", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
}
