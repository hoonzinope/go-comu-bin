package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
)

func AdminOnly(userRepository port.UserRepository, writeError func(*gin.Context, int, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		if userRepository == nil {
			writeError(c, http.StatusInternalServerError, customerror.Mark(customerror.ErrInternalServerError, "missing user repository"))
			return
		}
		userID, ok := UserID(c)
		if !ok {
			writeError(c, http.StatusUnauthorized, customerror.ErrUnauthorized)
			return
		}
		user, err := userRepository.SelectUserByID(c.Request.Context(), userID)
		if err != nil {
			writeError(c, http.StatusInternalServerError, customerror.WrapRepository("select user by id for admin middleware", err))
			return
		}
		if user == nil || !user.IsAdmin() {
			writeError(c, http.StatusForbidden, customerror.ErrForbidden)
			return
		}
		c.Next()
	}
}
