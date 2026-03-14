package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubUserRepository struct {
	selectByID func(ctx context.Context, id int64) (*entity.User, error)
}

func (s stubUserRepository) Save(context.Context, *entity.User) (int64, error) { return 0, nil }
func (s stubUserRepository) SelectUserByUsername(context.Context, string) (*entity.User, error) {
	return nil, nil
}
func (s stubUserRepository) SelectUserByUUID(context.Context, string) (*entity.User, error) {
	return nil, nil
}
func (s stubUserRepository) SelectUserByID(ctx context.Context, id int64) (*entity.User, error) {
	return s.selectByID(ctx, id)
}
func (s stubUserRepository) SelectUserByIDIncludingDeleted(context.Context, int64) (*entity.User, error) {
	return nil, nil
}
func (s stubUserRepository) SelectUsersByIDsIncludingDeleted(context.Context, []int64) (map[int64]*entity.User, error) {
	return nil, nil
}
func (s stubUserRepository) Update(context.Context, *entity.User) error { return nil }
func (s stubUserRepository) Delete(context.Context, int64) error        { return nil }

func TestAdminOnly_RejectsNonAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_id", int64(2))
		c.Next()
	})
	r.Use(AdminOnly(stubUserRepository{
		selectByID: func(ctx context.Context, id int64) (*entity.User, error) {
			return entity.NewUser("user", "pw"), nil
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
	r.Use(AdminOnly(stubUserRepository{
		selectByID: func(ctx context.Context, id int64) (*entity.User, error) {
			return entity.NewAdmin("admin", "pw"), nil
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
