package delivery

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
)

func RecoveryWithLogger(logger *slog.Logger) gin.HandlerFunc {
	return recoveryWithLogger(logger)
}

func recoveryWithLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}
			stack := debug.Stack()
			method := ""
			clientIP := ""
			userAgent := ""
			path := routePath(c)
			uri := ""
			if c != nil && c.Request != nil {
				method = c.Request.Method
				clientIP = c.ClientIP()
				userAgent = c.Request.UserAgent()
				if c.Request.URL != nil {
					uri = c.Request.URL.RequestURI()
				}
			}
			if logger != nil {
				logger.Error(
					"http handler panicked",
					"method", method,
					"path", path,
					"request_uri", uri,
					"client_ip", clientIP,
					"user_agent", userAgent,
					"panic", recovered,
					"stack", string(stack),
				)
			}
			if !c.Writer.Written() {
				c.AbortWithStatusJSON(http.StatusInternalServerError, errorResponse{Error: customerror.ErrInternalServerError.Error()})
				return
			}
			c.Abort()
		}()
		c.Next()
	}
}

func routePath(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if path := c.FullPath(); path != "" {
		return path
	}
	if c.Request != nil && c.Request.URL != nil {
		return c.Request.URL.Path
	}
	return ""
}
