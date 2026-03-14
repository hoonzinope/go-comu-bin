package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
)

func AdminOnly(userRepository port.UserRepository, writeError func(*gin.Context, int, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		if userRepository == nil {
			writeError(c, http.StatusInternalServerError, customError.Mark(customError.ErrInternalServerError, "missing user repository"))
			return
		}
		userID, ok := UserID(c)
		if !ok {
			writeError(c, http.StatusUnauthorized, customError.ErrUnauthorized)
			return
		}
		user, err := userRepository.SelectUserByID(c.Request.Context(), userID)
		if err != nil {
			writeError(c, http.StatusInternalServerError, customError.WrapRepository("select user by id for admin middleware", err))
			return
		}
		if user == nil || !user.IsAdmin() {
			writeError(c, http.StatusForbidden, customError.ErrForbidden)
			return
		}
		c.Next()
	}
}
