package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
)

func AdminOnly(adminAuthorizer port.AdminAuthorizer, writeError func(*gin.Context, int, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		if adminAuthorizer == nil {
			writeError(c, http.StatusInternalServerError, customerror.Mark(customerror.ErrInternalServerError, "missing admin authorizer"))
			return
		}
		userID, ok := UserID(c)
		if !ok {
			writeError(c, http.StatusUnauthorized, customerror.ErrUnauthorized)
			return
		}
		if err := adminAuthorizer.EnsureAdmin(c.Request.Context(), userID); err != nil {
			if errors.Is(err, customerror.ErrForbidden) || errors.Is(err, customerror.ErrUnauthorized) || errors.Is(err, customerror.ErrUserNotFound) {
				writeError(c, http.StatusForbidden, err)
				return
			}
			writeError(c, http.StatusInternalServerError, err)
			return
		}
		c.Next()
	}
}
