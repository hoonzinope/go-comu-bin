package delivery

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/delivery/middleware"
	"github.com/hoonzinope/go-comu-bin/internal/delivery/response"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

const multipartRequestOverheadBytes int64 = 1 << 20
const defaultMaxJSONBodyBytes int64 = 1 << 20
const defaultPageLimit = 10
const defaultRateLimitWindow = 60 * time.Second
const defaultRateLimitReadRequests = 300
const defaultRateLimitWriteRequests = 60
const defaultEmailVerificationRateLimitMaxRequests = 5
const defaultPasswordResetRateLimitMaxRequests = 5
const maxPageLimit = 1000
const httpLoggerContextKey = "http_logger"

type HTTPHandler struct {
	sessionUseCase                        port.SessionUseCase
	adminAuthorizer                       port.AdminAuthorizer
	userUseCase                           port.UserUseCase
	accountUseCase                        port.AccountUseCase
	boardUseCase                          port.BoardUseCase
	postUseCase                           port.PostUseCase
	commentUseCase                        port.CommentUseCase
	notificationUseCase                   port.NotificationUseCase
	reactionUseCase                       port.ReactionUseCase
	attachmentUseCase                     port.AttachmentUseCase
	reportUseCase                         port.ReportUseCase
	outboxAdminUseCase                    port.OutboxAdminUseCase
	rateLimiter                           port.RateLimiter
	attachmentUploadMaxBytes              int64
	maxJSONBodyBytes                      int64
	defaultPageLimit                      int
	rateLimitEnabled                      bool
	rateLimitWindow                       time.Duration
	rateLimitReadRequests                 int
	rateLimitWriteRequests                int
	emailVerificationRateLimitEnabled     bool
	emailVerificationRateLimitWindow      time.Duration
	emailVerificationRateLimitMaxRequests int
	passwordResetRateLimitEnabled         bool
	passwordResetRateLimitWindow          time.Duration
	passwordResetRateLimitMaxRequests     int
	logger                                *slog.Logger
	authGinMiddleware                     gin.HandlerFunc
	adminGinMiddleware                    gin.HandlerFunc
}

type HTTPDependencies struct {
	SessionUseCase                         port.SessionUseCase
	AdminAuthorizer                        port.AdminAuthorizer
	UserUseCase                            port.UserUseCase
	AccountUseCase                         port.AccountUseCase
	BoardUseCase                           port.BoardUseCase
	PostUseCase                            port.PostUseCase
	CommentUseCase                         port.CommentUseCase
	NotificationUseCase                    port.NotificationUseCase
	ReactionUseCase                        port.ReactionUseCase
	AttachmentUseCase                      port.AttachmentUseCase
	ReportUseCase                          port.ReportUseCase
	OutboxAdminUseCase                     port.OutboxAdminUseCase
	RateLimiter                            port.RateLimiter
	AttachmentUploadMaxBytes               int64
	MaxJSONBodyBytes                       int64
	DefaultPageLimit                       int
	RateLimitEnabled                       bool
	RateLimitWindowSecond                  int
	RateLimitReadRequest                   int
	RateLimitWriteRequest                  int
	EmailVerificationRateLimitEnabled      bool
	EmailVerificationRateLimitWindowSecond int
	EmailVerificationRateLimitMaxRequests  int
	PasswordResetRateLimitEnabled          bool
	PasswordResetRateLimitWindowSecond     int
	PasswordResetRateLimitMaxRequests      int
	Logger                                 *slog.Logger
}

func NewHTTPHandler(deps HTTPDependencies) *HTTPHandler {
	logger := deps.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	handler := &HTTPHandler{
		sessionUseCase:                        deps.SessionUseCase,
		adminAuthorizer:                       deps.AdminAuthorizer,
		userUseCase:                           deps.UserUseCase,
		accountUseCase:                        deps.AccountUseCase,
		boardUseCase:                          deps.BoardUseCase,
		postUseCase:                           deps.PostUseCase,
		commentUseCase:                        deps.CommentUseCase,
		notificationUseCase:                   deps.NotificationUseCase,
		reactionUseCase:                       deps.ReactionUseCase,
		attachmentUseCase:                     deps.AttachmentUseCase,
		reportUseCase:                         deps.ReportUseCase,
		outboxAdminUseCase:                    deps.OutboxAdminUseCase,
		rateLimiter:                           deps.RateLimiter,
		attachmentUploadMaxBytes:              deps.AttachmentUploadMaxBytes,
		maxJSONBodyBytes:                      resolveMaxJSONBodyBytes(deps.MaxJSONBodyBytes),
		defaultPageLimit:                      resolveDefaultPageLimit(deps.DefaultPageLimit),
		rateLimitEnabled:                      deps.RateLimitEnabled,
		rateLimitWindow:                       resolveRateLimitWindow(deps.RateLimitWindowSecond),
		rateLimitReadRequests:                 resolveRateLimitReadRequests(deps.RateLimitReadRequest),
		rateLimitWriteRequests:                resolveRateLimitWriteRequests(deps.RateLimitWriteRequest),
		emailVerificationRateLimitEnabled:     deps.EmailVerificationRateLimitEnabled,
		emailVerificationRateLimitWindow:      resolveRateLimitWindow(deps.EmailVerificationRateLimitWindowSecond),
		emailVerificationRateLimitMaxRequests: resolveEmailVerificationRateLimitMaxRequests(deps.EmailVerificationRateLimitMaxRequests),
		passwordResetRateLimitEnabled:         deps.PasswordResetRateLimitEnabled,
		passwordResetRateLimitWindow:          resolveRateLimitWindow(deps.PasswordResetRateLimitWindowSecond),
		passwordResetRateLimitMaxRequests:     resolvePasswordResetRateLimitMaxRequests(deps.PasswordResetRateLimitMaxRequests),
		logger:                                logger,
	}
	handler.authGinMiddleware = middleware.AuthWithSession(deps.SessionUseCase, func(c *gin.Context, status int, err error) {
		writeHTTPError(handler.logger, c, status, err)
	})
	handler.adminGinMiddleware = middleware.AdminOnly(deps.AdminAuthorizer, func(c *gin.Context, status int, err error) {
		writeHTTPError(handler.logger, c, status, err)
	})
	return handler
}

func resolveMaxJSONBodyBytes(size int64) int64 {
	if size <= 0 {
		return defaultMaxJSONBodyBytes
	}
	return size
}

func resolveDefaultPageLimit(limit int) int {
	if limit < 1 || limit > maxPageLimit {
		return defaultPageLimit
	}
	return limit
}

func resolveRateLimitWindow(windowSeconds int) time.Duration {
	if windowSeconds <= 0 {
		return defaultRateLimitWindow
	}
	return time.Duration(windowSeconds) * time.Second
}

func resolveRateLimitWriteRequests(requests int) int {
	if requests <= 0 {
		return defaultRateLimitWriteRequests
	}
	return requests
}

func resolveRateLimitReadRequests(requests int) int {
	if requests <= 0 {
		return defaultRateLimitReadRequests
	}
	return requests
}

func resolvePasswordResetRateLimitMaxRequests(requests int) int {
	if requests <= 0 {
		return defaultPasswordResetRateLimitMaxRequests
	}
	return requests
}

func resolveEmailVerificationRateLimitMaxRequests(requests int) int {
	if requests <= 0 {
		return defaultEmailVerificationRateLimitMaxRequests
	}
	return requests
}

func (h *HTTPHandler) RegisterRoutes(r *gin.Engine) {
	r.Use(func(c *gin.Context) {
		c.Set(httpLoggerContextKey, h.logger)
		c.Next()
	})
	r.HandleMethodNotAllowed = true
	r.NoMethod(func(c *gin.Context) {
		writeHTTPError(h.logger, c, http.StatusMethodNotAllowed, customerror.ErrMethodNotAllowed)
	})
	r.NoRoute(func(c *gin.Context) {
		writeHTTPError(h.logger, c, http.StatusNotFound, customerror.ErrNotFound)
	})
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	v1 := r.Group("/api/v1")
	v1.Use(h.rateLimitGinMiddleware())
	v1.POST("/signup", h.handleUserSignUp)
	v1.POST("/auth/login", h.handleUserLogin)
	v1.POST("/auth/guest", h.handleGuestIssue)
	v1.POST("/auth/guest/upgrade", h.authGinMiddleware, h.handleGuestUpgrade)
	v1.POST("/auth/email-verification/request", h.authGinMiddleware, h.handleEmailVerificationRequest)
	v1.POST("/auth/email-verification/confirm", h.handleEmailVerificationConfirm)
	v1.POST("/auth/password-reset/request", h.handlePasswordResetRequest)
	v1.POST("/auth/password-reset/confirm", h.handlePasswordResetConfirm)
	v1.POST("/auth/logout", h.authGinMiddleware, h.handleUserLogout)
	v1.DELETE("/users/me", h.authGinMiddleware, h.handleUserDeleteMe)
	v1.GET("/users/me/notifications", h.authGinMiddleware, h.handleMyNotificationsGet)
	v1.GET("/users/me/notifications/unread-count", h.authGinMiddleware, h.handleMyNotificationsUnreadCountGet)
	v1.PATCH("/users/me/notifications/:notificationUUID/read", h.authGinMiddleware, h.handleMyNotificationReadPatch)
	v1.POST("/reports", h.authGinMiddleware, h.handleReportCreate)
	v1.GET("/users/:userUUID/suspension", h.authGinMiddleware, h.adminGinMiddleware, h.handleUserSuspensionGet)
	v1.PUT("/users/:userUUID/suspension", h.authGinMiddleware, h.adminGinMiddleware, h.handleUserSuspend)
	v1.DELETE("/users/:userUUID/suspension", h.authGinMiddleware, h.adminGinMiddleware, h.handleUserUnsuspend)

	v1.GET("/boards", h.handleBoardsGet)
	v1.POST("/boards", h.authGinMiddleware, h.handleBoardsPost)
	v1.PUT("/boards/:boardUUID", h.authGinMiddleware, h.handleBoardPut)
	v1.DELETE("/boards/:boardUUID", h.authGinMiddleware, h.handleBoardDelete)

	v1.GET("/boards/:boardUUID/posts", h.handleBoardPostsGet)
	v1.POST("/boards/:boardUUID/posts", h.authGinMiddleware, h.handleBoardPostsPost)
	v1.POST("/boards/:boardUUID/posts/drafts", h.authGinMiddleware, h.handleBoardDraftPostsPost)
	v1.GET("/tags/:tagName/posts", h.handleTagPostsGet)

	v1.GET("/posts/feed", h.handlePostFeedGet)
	v1.GET("/posts/search", h.handlePostSearchGet)
	v1.GET("/posts/:postUUID", h.handlePostDetailGet)
	v1.POST("/posts/:postUUID/publish", h.authGinMiddleware, h.handlePostPublish)
	v1.GET("/posts/:postUUID/attachments", h.handlePostAttachmentsGet)
	v1.GET("/posts/:postUUID/attachments/:attachmentUUID/file", h.handlePostAttachmentFileGet)
	v1.GET("/posts/:postUUID/attachments/:attachmentUUID/preview", h.authGinMiddleware, h.handlePostAttachmentPreviewGet)
	v1.POST("/posts/:postUUID/attachments/upload", h.authGinMiddleware, h.handlePostAttachmentsUpload)
	v1.DELETE("/posts/:postUUID/attachments/:attachmentUUID", h.authGinMiddleware, h.handlePostAttachmentDelete)
	v1.PUT("/posts/:postUUID", h.authGinMiddleware, h.handlePostDetailPut)
	v1.DELETE("/posts/:postUUID", h.authGinMiddleware, h.handlePostDetailDelete)

	v1.GET("/posts/:postUUID/comments", h.handlePostCommentsGet)
	v1.POST("/posts/:postUUID/comments", h.authGinMiddleware, h.handlePostCommentsPost)
	v1.GET("/posts/:postUUID/reactions", h.handlePostReactions)
	v1.PUT("/posts/:postUUID/reactions/me", h.authGinMiddleware, h.handleMyPostReactionPut)
	v1.DELETE("/posts/:postUUID/reactions/me", h.authGinMiddleware, h.handleMyPostReactionDelete)

	v1.PUT("/comments/:commentUUID", h.authGinMiddleware, h.handleCommentPut)
	v1.DELETE("/comments/:commentUUID", h.authGinMiddleware, h.handleCommentDelete)
	v1.GET("/comments/:commentUUID/reactions", h.handleCommentReactions)
	v1.PUT("/comments/:commentUUID/reactions/me", h.authGinMiddleware, h.handleMyCommentReactionPut)
	v1.DELETE("/comments/:commentUUID/reactions/me", h.authGinMiddleware, h.handleMyCommentReactionDelete)

	admin := v1.Group("/admin", h.authGinMiddleware, h.adminGinMiddleware)
	admin.GET("/reports", h.handleAdminReportsGet)
	admin.PUT("/reports/:reportID/resolve", h.handleAdminReportResolve)
	admin.GET("/outbox/dead", h.handleAdminDeadOutboxGet)
	admin.POST("/outbox/dead/:messageID/requeue", h.handleAdminDeadOutboxRequeue)
	admin.DELETE("/outbox/dead/:messageID", h.handleAdminDeadOutboxDiscard)
	admin.PUT("/boards/:boardUUID/visibility", h.handleAdminBoardVisibilityPut)
}

func NewHTTPServer(addr string, deps HTTPDependencies) *http.Server {
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	r.Use(gin.Recovery())
	if deps.AttachmentUploadMaxBytes > 0 {
		r.MaxMultipartMemory = deps.AttachmentUploadMaxBytes + multipartRequestOverheadBytes
	}
	handler := NewHTTPHandler(deps)
	handler.RegisterRoutes(r)
	return &http.Server{Addr: addr, Handler: r}
}

func (h *HTTPHandler) requireAuthUserID(c *gin.Context) (int64, bool) {
	userID, ok := middleware.UserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse{Error: customerror.ErrUnauthorized.Error()})
		return 0, false
	}
	return userID, true
}

// handleUserSignUp godoc
// @Summary Sign up
// @Description Create a new user account.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body signUpRequest true "Sign up payload"
// @Success 201 {object} signUpResponse
// @Failure 400 {object} errorResponse
// @Failure 409 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /signup [post]
func (h *HTTPHandler) handleUserSignUp(c *gin.Context) {
	var req signUpRequest
	if err := h.decodeJSON(c, &req); err != nil {
		badRequest(c, err)
		return
	}
	if err := req.validate(); err != nil {
		badRequest(c, err)
		return
	}
	if _, err := h.userUseCase.SignUp(c.Request.Context(), req.Username, req.Email, req.Password); err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.JSON(http.StatusCreated, signUpResponse{Result: "ok"})
}

// handleUserLogin godoc
// @Summary Login
// @Description Authenticate user and return bearer token in Authorization response header.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body userCredentialRequest true "Login payload"
// @Success 200 {object} loginResponse
// @Header 200 {string} Authorization "Bearer <token>"
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /auth/login [post]
func (h *HTTPHandler) handleUserLogin(c *gin.Context) {
	var req userCredentialRequest
	if err := h.decodeJSON(c, &req); err != nil {
		badRequest(c, err)
		return
	}
	if err := req.validate(); err != nil {
		badRequest(c, err)
		return
	}
	token, err := h.sessionUseCase.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.Header("Authorization", "Bearer "+token)
	c.JSON(http.StatusOK, loginResponse{Login: "ok"})
}

// handleGuestIssue godoc
// @Summary Issue Guest Token
// @Description Create a server-generated guest account and return bearer token in Authorization response header.
// @Tags Auth
// @Accept json
// @Produce json
// @Success 201 {object} loginResponse
// @Header 201 {string} Authorization "Bearer <token>"
// @Failure 500 {object} errorResponse
// @Router /auth/guest [post]
func (h *HTTPHandler) handleGuestIssue(c *gin.Context) {
	token, err := h.sessionUseCase.IssueGuestToken(c.Request.Context())
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.Header("Authorization", "Bearer "+token)
	c.JSON(http.StatusCreated, loginResponse{Login: "ok"})
}

// handleGuestUpgrade godoc
// @Summary Upgrade Guest Account
// @Description Upgrade current guest account into a regular account.
// @Tags Auth
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body guestUpgradeRequest true "Guest upgrade payload"
// @Success 200 {object} signUpResponse
// @Header 200 {string} Authorization "Bearer <token>"
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 409 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /auth/guest/upgrade [post]
func (h *HTTPHandler) handleGuestUpgrade(c *gin.Context) {
	userID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	currentToken, ok := middleware.Token(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse{Error: customerror.ErrUnauthorized.Error()})
		return
	}
	var req guestUpgradeRequest
	if err := h.decodeJSON(c, &req); err != nil {
		badRequest(c, err)
		return
	}
	if err := req.validate(); err != nil {
		badRequest(c, err)
		return
	}
	token, err := h.accountUseCase.UpgradeGuestAccount(c.Request.Context(), userID, currentToken, req.Username, req.Email, req.Password)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.Header("Authorization", "Bearer "+token)
	c.JSON(http.StatusOK, signUpResponse{Result: "ok"})
}

// handlePasswordResetRequest godoc
// @Summary Request Email Verification
// @Description Create or resend a one-time email verification token for the authenticated user. The token is delivered through the configured mail sender with a frontend verification link.
// @Tags Auth
// @Security BearerAuth
// @Accept json
// @Produce json
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 429 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /auth/email-verification/request [post]
func (h *HTTPHandler) handleEmailVerificationRequest(c *gin.Context) {
	userID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	limited, err := h.enforceEmailVerificationRequestRateLimit(c, userID)
	if err != nil {
		writeHTTPError(h.logger, c, http.StatusInternalServerError, customerror.Wrap(customerror.ErrInternalServerError, "email verification request rate limit", err))
		return
	}
	if limited {
		h.logger.Info(
			"email verification request audit",
			"event", "email_verification_request",
			"user_id", userID,
			"outcome", "rate_limited",
		)
		return
	}
	if err := h.accountUseCase.RequestEmailVerification(c.Request.Context(), userID); err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *HTTPHandler) enforceEmailVerificationRequestRateLimit(c *gin.Context, userID int64) (bool, error) {
	if h == nil || !h.emailVerificationRateLimitEnabled || h.rateLimiter == nil {
		return false, nil
	}
	allowed, err := h.rateLimiter.Allow(
		c.Request.Context(),
		emailVerificationRateLimitKey(userID),
		h.emailVerificationRateLimitMaxRequests,
		h.emailVerificationRateLimitWindow,
	)
	if err != nil {
		return false, err
	}
	if allowed {
		return false, nil
	}
	writeHTTPError(h.logger, c, http.StatusTooManyRequests, customerror.ErrTooManyRequests)
	return true, nil
}

// handleEmailVerificationConfirm godoc
// @Summary Confirm Email Verification
// @Description Verify the account email using a valid one-time token.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body emailVerificationConfirmRequest true "Email verification confirm payload"
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /auth/email-verification/confirm [post]
func (h *HTTPHandler) handleEmailVerificationConfirm(c *gin.Context) {
	var req emailVerificationConfirmRequest
	if err := h.decodeJSON(c, &req); err != nil {
		badRequest(c, err)
		return
	}
	if err := req.validate(); err != nil {
		badRequest(c, err)
		return
	}
	if err := h.accountUseCase.ConfirmEmailVerification(c.Request.Context(), req.Token); err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// handlePasswordResetRequest godoc
// @Summary Request Password Reset
// @Description Create a one-time password reset token and deliver it through the configured mail sender.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body passwordResetRequest true "Password reset request payload"
// @Success 200 {object} signUpResponse
// @Failure 400 {object} errorResponse
// @Failure 429 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /auth/password-reset/request [post]
func (h *HTTPHandler) handlePasswordResetRequest(c *gin.Context) {
	var req passwordResetRequest
	if err := h.decodeJSON(c, &req); err != nil {
		badRequest(c, err)
		return
	}
	if err := req.validate(); err != nil {
		badRequest(c, err)
		return
	}
	if limited, err := h.enforcePasswordResetRequestRateLimit(c, req.Email); err != nil {
		writeHTTPError(h.logger, c, http.StatusInternalServerError, customerror.Wrap(customerror.ErrInternalServerError, "password reset request rate limit", err))
		return
	} else if limited {
		return
	}
	if err := h.accountUseCase.RequestPasswordReset(c.Request.Context(), req.Email); err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.JSON(http.StatusOK, signUpResponse{Result: "ok"})
}

func (h *HTTPHandler) enforcePasswordResetRequestRateLimit(c *gin.Context, email string) (bool, error) {
	if h == nil || !h.passwordResetRateLimitEnabled || h.rateLimiter == nil {
		return false, nil
	}
	allowed, err := h.rateLimiter.Allow(
		c.Request.Context(),
		passwordResetRateLimitKey(c.ClientIP(), email),
		h.passwordResetRateLimitMaxRequests,
		h.passwordResetRateLimitWindow,
	)
	if err != nil {
		return false, err
	}
	if allowed {
		return false, nil
	}
	writeHTTPError(h.logger, c, http.StatusTooManyRequests, customerror.ErrTooManyRequests)
	return true, nil
}

func emailVerificationRateLimitKey(userID int64) string {
	return fmt.Sprintf("email-verification-request:user:%d", userID)
}

// handlePasswordResetConfirm godoc
// @Summary Confirm Password Reset
// @Description Reset the account password using a valid one-time token.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body passwordResetConfirmRequest true "Password reset confirm payload"
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /auth/password-reset/confirm [post]
func (h *HTTPHandler) handlePasswordResetConfirm(c *gin.Context) {
	var req passwordResetConfirmRequest
	if err := h.decodeJSON(c, &req); err != nil {
		badRequest(c, err)
		return
	}
	if err := req.validate(); err != nil {
		badRequest(c, err)
		return
	}
	if err := h.accountUseCase.ConfirmPasswordReset(c.Request.Context(), req.Token, req.NewPassword); err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// handleUserLogout godoc
// @Summary Logout
// @Description Invalidate current token in cache.
// @Tags Auth
// @Security BearerAuth
// @Accept json
// @Produce json
// @Success 200 {object} logoutResponse
// @Failure 401 {object} errorResponse
// @Router /auth/logout [post]
func (h *HTTPHandler) handleUserLogout(c *gin.Context) {
	if _, ok := h.requireAuthUserID(c); !ok {
		return
	}
	token, ok := middleware.Token(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse{Error: customerror.ErrUnauthorized.Error()})
		return
	}
	if err := h.sessionUseCase.Logout(c.Request.Context(), token); err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.JSON(http.StatusOK, logoutResponse{Logout: "ok"})
}

// handleUserDeleteMe godoc
// @Summary Delete My Account
// @Description Delete the authenticated user account with password confirmation.
// @Tags User
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body passwordOnlyRequest true "Password confirmation"
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /users/me [delete]
func (h *HTTPHandler) handleUserDeleteMe(c *gin.Context) {
	userID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	var req passwordOnlyRequest
	if err := h.decodeJSON(c, &req); err != nil {
		badRequest(c, err)
		return
	}
	if err := req.validate(); err != nil {
		badRequest(c, err)
		return
	}
	if err := h.accountUseCase.DeleteMyAccount(c.Request.Context(), userID, req.Password); err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// handleMyNotificationsGet godoc
// @Summary List My Notifications
// @Description Returns notifications for the authenticated user with cursor pagination.
// @Tags Notification
// @Security BearerAuth
// @Produce json
// @Param limit query int false "Page size"
// @Param cursor query string false "Opaque cursor"
// @Success 200 {object} response.NotificationList
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /users/me/notifications [get]
func (h *HTTPHandler) handleMyNotificationsGet(c *gin.Context) {
	userID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	limit, cursor, ok := h.parseLimitCursor(c)
	if !ok {
		return
	}
	list, err := h.notificationUseCase.GetMyNotifications(c.Request.Context(), userID, limit, cursor)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.NotificationListFromDTO(list))
}

// handleMyNotificationsUnreadCountGet godoc
// @Summary Get My Notification Unread Count
// @Description Returns unread notification count for the authenticated user.
// @Tags Notification
// @Security BearerAuth
// @Produce json
// @Success 200 {object} countResponse
// @Failure 401 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /users/me/notifications/unread-count [get]
func (h *HTTPHandler) handleMyNotificationsUnreadCountGet(c *gin.Context) {
	userID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	count, err := h.notificationUseCase.GetMyUnreadNotificationCount(c.Request.Context(), userID)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.JSON(http.StatusOK, countResponse{Count: count})
}

// handleMyNotificationReadPatch godoc
// @Summary Mark My Notification Read
// @Description Marks one notification as read for the authenticated user.
// @Tags Notification
// @Security BearerAuth
// @Produce json
// @Param notificationUUID path string true "Notification UUID" format(uuid)
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /users/me/notifications/{notificationUUID}/read [patch]
func (h *HTTPHandler) handleMyNotificationReadPatch(c *gin.Context) {
	userID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	notificationUUID, ok := parsePathUUID(c, "notificationUUID", "notification")
	if !ok {
		return
	}
	if err := h.notificationUseCase.MarkMyNotificationRead(c.Request.Context(), userID, notificationUUID); err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// handleReportCreate godoc
// @Summary Create Report
// @Description Create a report for a post or comment.
// @Tags Report
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body reportCreateRequest true "Report payload"
// @Success 201 {object} idResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 409 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /reports [post]
func (h *HTTPHandler) handleReportCreate(c *gin.Context) {
	userID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	var req reportCreateRequest
	if err := h.decodeJSON(c, &req); err != nil {
		badRequest(c, err)
		return
	}
	targetType, targetID, reasonCode, reasonDetail, err := req.parse()
	if err != nil {
		badRequest(c, err)
		return
	}
	id, err := h.reportUseCase.CreateReport(c.Request.Context(), userID, targetType, targetID, reasonCode, reasonDetail)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.JSON(http.StatusCreated, idResponse{ID: id})
}

// handleUserSuspensionGet godoc
// @Summary Get User Suspension
// @Description Returns the current suspension status for a user (admin only).
// @Tags User
// @Security BearerAuth
// @Produce json
// @Param userUUID path string true "User UUID"
// @Success 200 {object} userSuspensionResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /users/{userUUID}/suspension [get]
func (h *HTTPHandler) handleUserSuspensionGet(c *gin.Context) {
	targetUserUUID, ok := parsePathUUID(c, "userUUID", "user")
	if !ok {
		return
	}
	adminID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	view, err := h.userUseCase.GetUserSuspension(c.Request.Context(), adminID, targetUserUUID)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.JSON(http.StatusOK, userSuspensionResponse{
		UserUUID:       view.UserUUID,
		Status:         string(view.Status),
		Reason:         view.Reason,
		SuspendedUntil: view.SuspendedUntil,
	})
}

// handleUserSuspend godoc
// @Summary Suspend User
// @Description Suspend a user from post/comment write actions (admin only).
// @Tags User
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param userUUID path string true "User UUID"
// @Param request body userSuspensionRequest true "Suspension payload"
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /users/{userUUID}/suspension [put]
func (h *HTTPHandler) handleUserSuspend(c *gin.Context) {
	targetUserUUID, ok := parsePathUUID(c, "userUUID", "user")
	if !ok {
		return
	}
	adminID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	var req userSuspensionRequest
	if err := h.decodeJSON(c, &req); err != nil {
		badRequest(c, err)
		return
	}
	reason, duration, err := req.parse()
	if err != nil {
		badRequest(c, err)
		return
	}
	if err := h.userUseCase.SuspendUser(c.Request.Context(), adminID, targetUserUUID, reason, duration); err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// handleUserUnsuspend godoc
// @Summary Unsuspend User
// @Description Clear a user's suspension (admin only).
// @Tags User
// @Security BearerAuth
// @Produce json
// @Param userUUID path string true "User UUID"
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /users/{userUUID}/suspension [delete]
func (h *HTTPHandler) handleUserUnsuspend(c *gin.Context) {
	targetUserUUID, ok := parsePathUUID(c, "userUUID", "user")
	if !ok {
		return
	}
	adminID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	if err := h.userUseCase.UnsuspendUser(c.Request.Context(), adminID, targetUserUUID); err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// handleAdminReportsGet godoc
// @Summary List Reports
// @Description List reports (admin only).
// @Tags Admin
// @Produce json
// @Security BearerAuth
// @Param status query string false "Report status filter"
// @Param limit query int false "Page size" minimum(1) maximum(1000)
// @Param last_id query int false "Cursor id" minimum(0)
// @Success 200 {object} reportListResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /admin/reports [get]
func (h *HTTPHandler) handleAdminReportsGet(c *gin.Context) {
	adminID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	limit, lastID, ok := h.parseLimitLastID(c)
	if !ok {
		return
	}
	var status *model.ReportStatus
	if raw := strings.TrimSpace(c.Query("status")); raw != "" {
		parsed, parseOK := model.ParseReportStatus(raw)
		if !parseOK {
			badRequest(c, errors.New("invalid status"))
			return
		}
		status = &parsed
	}
	list, err := h.reportUseCase.GetReports(c.Request.Context(), adminID, status, limit, lastID)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.JSON(http.StatusOK, reportListResponse{
		Reports:    reportResponsesFromModel(list.Reports),
		Limit:      list.Limit,
		LastID:     list.LastID,
		HasMore:    list.HasMore,
		NextLastID: list.NextLastID,
	})
}

// handleAdminReportResolve godoc
// @Summary Resolve Report
// @Description Resolve report with accepted/rejected status (admin only).
// @Tags Admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param reportID path int true "Report ID"
// @Param request body reportResolveRequest true "Resolve payload"
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /admin/reports/{reportID}/resolve [put]
func (h *HTTPHandler) handleAdminReportResolve(c *gin.Context) {
	adminID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	reportID, ok := parsePathID(c, "reportID", "report")
	if !ok {
		return
	}
	var req reportResolveRequest
	if err := h.decodeJSON(c, &req); err != nil {
		badRequest(c, err)
		return
	}
	status, resolutionNote, err := req.parseStatus()
	if err != nil {
		badRequest(c, err)
		return
	}
	if err := h.reportUseCase.ResolveReport(c.Request.Context(), adminID, reportID, status, resolutionNote); err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// handleAdminDeadOutboxGet godoc
// @Summary List Dead Outbox Messages
// @Description List dead outbox messages (admin only).
// @Tags Admin
// @Produce json
// @Security BearerAuth
// @Param limit query int false "Page size" minimum(1) maximum(1000)
// @Param last_id query string false "Cursor id"
// @Success 200 {object} outboxDeadListResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /admin/outbox/dead [get]
func (h *HTTPHandler) handleAdminDeadOutboxGet(c *gin.Context) {
	adminID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	limit, lastID, ok := h.parseLimitLastIDString(c)
	if !ok {
		return
	}
	list, err := h.outboxAdminUseCase.GetDeadMessages(c.Request.Context(), adminID, limit, lastID)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.JSON(http.StatusOK, outboxDeadListResponse{
		Messages:   outboxDeadResponsesFromModel(list.Messages),
		Limit:      list.Limit,
		LastID:     list.LastID,
		HasMore:    list.HasMore,
		NextLastID: list.NextLastID,
	})
}

// handleAdminDeadOutboxRequeue godoc
// @Summary Requeue Dead Outbox Message
// @Description Requeue one dead outbox message (admin only).
// @Tags Admin
// @Produce json
// @Security BearerAuth
// @Param messageID path string true "Outbox Message ID"
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /admin/outbox/dead/{messageID}/requeue [post]
func (h *HTTPHandler) handleAdminDeadOutboxRequeue(c *gin.Context) {
	adminID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	messageID := strings.TrimSpace(c.Param("messageID"))
	if messageID == "" {
		badRequest(c, errors.New("invalid message id"))
		return
	}
	if err := h.outboxAdminUseCase.RequeueDeadMessage(c.Request.Context(), adminID, messageID); err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// handleAdminDeadOutboxDiscard godoc
// @Summary Discard Dead Outbox Message
// @Description Permanently discard one dead outbox message (admin only).
// @Tags Admin
// @Produce json
// @Security BearerAuth
// @Param messageID path string true "Outbox Message ID"
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /admin/outbox/dead/{messageID} [delete]
func (h *HTTPHandler) handleAdminDeadOutboxDiscard(c *gin.Context) {
	adminID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	messageID := strings.TrimSpace(c.Param("messageID"))
	if messageID == "" {
		badRequest(c, errors.New("invalid message id"))
		return
	}
	if err := h.outboxAdminUseCase.DiscardDeadMessage(c.Request.Context(), adminID, messageID); err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// handleAdminBoardVisibilityPut godoc
// @Summary Set Board Visibility
// @Description Set board hidden visibility (admin only).
// @Tags Admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param boardUUID path string true "Board UUID" format(uuid)
// @Param request body boardVisibilityRequest true "Visibility payload"
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /admin/boards/{boardUUID}/visibility [put]
func (h *HTTPHandler) handleAdminBoardVisibilityPut(c *gin.Context) {
	adminID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	boardUUID, ok := parsePathUUID(c, "boardUUID", "board")
	if !ok {
		return
	}
	var req boardVisibilityRequest
	if err := h.decodeJSON(c, &req); err != nil {
		badRequest(c, err)
		return
	}
	if err := h.boardUseCase.SetBoardVisibility(c.Request.Context(), boardUUID, adminID, req.Hidden); err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// handleBoardsGet godoc
// @Summary List Boards
// @Description Returns board list with cursor pagination.
// @Tags Board
// @Produce json
// @Param limit query int false "Page size" minimum(1) maximum(1000)
// @Param cursor query string false "Opaque cursor returned by previous list response"
// @Success 200 {object} response.BoardList
// @Failure 400 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /boards [get]
func (h *HTTPHandler) handleBoardsGet(c *gin.Context) {
	limit, cursor, ok := h.parseLimitCursor(c)
	if !ok {
		return
	}
	boards, err := h.boardUseCase.GetBoards(c.Request.Context(), limit, cursor)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.BoardListFromDTO(boards))
}

// handleBoardsPost godoc
// @Summary Create Board
// @Description Creates a board (admin only).
// @Tags Board
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body boardRequest true "Create board payload"
// @Success 201 {object} uuidResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /boards [post]
func (h *HTTPHandler) handleBoardsPost(c *gin.Context) {
	userID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	var req boardRequest
	if err := h.decodeJSON(c, &req); err != nil {
		badRequest(c, err)
		return
	}
	if err := req.validate(); err != nil {
		badRequest(c, err)
		return
	}
	id, err := h.boardUseCase.CreateBoard(c.Request.Context(), userID, req.Name, req.Description)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.JSON(http.StatusCreated, uuidResponse{UUID: id})
}

// handleBoardPut godoc
// @Summary Update Board
// @Description Updates a board by UUID (admin only).
// @Tags Board
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param boardUUID path string true "Board UUID" format(uuid)
// @Param request body boardRequest true "Update board payload"
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /boards/{boardUUID} [put]
func (h *HTTPHandler) handleBoardPut(c *gin.Context) {
	boardUUID, ok := parsePathUUID(c, "boardUUID", "board")
	if !ok {
		return
	}

	var req boardRequest
	if err := h.decodeJSON(c, &req); err != nil {
		badRequest(c, err)
		return
	}
	userID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	if err := req.validate(); err != nil {
		badRequest(c, err)
		return
	}
	if err := h.boardUseCase.UpdateBoard(c.Request.Context(), boardUUID, userID, req.Name, req.Description); err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// handleBoardDelete godoc
// @Summary Delete Board
// @Description Deletes a board by UUID (admin only).
// @Tags Board
// @Produce json
// @Security BearerAuth
// @Param boardUUID path string true "Board UUID" format(uuid)
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /boards/{boardUUID} [delete]
func (h *HTTPHandler) handleBoardDelete(c *gin.Context) {
	boardUUID, ok := parsePathUUID(c, "boardUUID", "board")
	if !ok {
		return
	}
	userID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	if err := h.boardUseCase.DeleteBoard(c.Request.Context(), boardUUID, userID); err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// handleBoardPostsGet godoc
// @Summary List Posts by Board
// @Description Returns published posts in a board with cursor pagination. Supports latest, hot, best, and top ordering.
// @Tags Post
// @Produce json
// @Param boardUUID path string true "Board UUID" format(uuid)
// @Param sort query string false "Board sort: hot, best, latest, top"
// @Param window query string false "Top window: 24h, 7d, 30d, all (allowed only when sort=top)"
// @Param limit query int false "Page size" minimum(1) maximum(1000)
// @Param cursor query string false "Opaque cursor returned by previous list response"
// @Success 200 {object} response.PostList
// @Failure 400 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /boards/{boardUUID}/posts [get]
func (h *HTTPHandler) handleBoardPostsGet(c *gin.Context) {
	boardUUID, ok := parsePathUUID(c, "boardUUID", "board")
	if !ok {
		return
	}
	sortBy := strings.TrimSpace(c.Query("sort"))
	window := strings.TrimSpace(c.Query("window"))
	limit, cursor, ok := h.parseLimitCursor(c)
	if !ok {
		return
	}
	posts, err := h.postUseCase.GetPostsList(c.Request.Context(), boardUUID, sortBy, window, limit, cursor)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.PostListFromDTO(posts))
}

// handleBoardPostsPost godoc
// @Summary Create Post
// @Description Creates a post in a board identified by UUID.
// @Tags Post
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param boardUUID path string true "Board UUID" format(uuid)
// @Param request body postRequest true "Create post payload"
// @Success 201 {object} uuidResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /boards/{boardUUID}/posts [post]
func (h *HTTPHandler) handleBoardPostsPost(c *gin.Context) {
	boardUUID, ok := parsePathUUID(c, "boardUUID", "board")
	if !ok {
		return
	}
	authorID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	var req postRequest
	if err := h.decodeJSON(c, &req); err != nil {
		badRequest(c, err)
		return
	}
	if err := req.validate(); err != nil {
		badRequest(c, err)
		return
	}
	postID, err := h.postUseCase.CreatePost(c.Request.Context(), req.Title, req.Content, req.Tags, req.MentionedUsernames, authorID, boardUUID)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.JSON(http.StatusCreated, uuidResponse{UUID: postID})
}

// handleBoardDraftPostsPost godoc
// @Summary Create Draft Post
// @Description Creates a draft post in a board identified by UUID.
// @Tags Post
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param boardUUID path string true "Board UUID" format(uuid)
// @Param request body postRequest true "Create draft post payload"
// @Success 201 {object} uuidResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /boards/{boardUUID}/posts/drafts [post]
func (h *HTTPHandler) handleBoardDraftPostsPost(c *gin.Context) {
	boardUUID, ok := parsePathUUID(c, "boardUUID", "board")
	if !ok {
		return
	}
	authorID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	var req postRequest
	if err := h.decodeJSON(c, &req); err != nil {
		badRequest(c, err)
		return
	}
	if err := req.validate(); err != nil {
		badRequest(c, err)
		return
	}
	postID, err := h.postUseCase.CreateDraftPost(c.Request.Context(), req.Title, req.Content, req.Tags, req.MentionedUsernames, authorID, boardUUID)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.JSON(http.StatusCreated, uuidResponse{UUID: postID})
}

// handleTagPostsGet godoc
// @Summary List Posts by Tag
// @Description Returns published posts connected to a tag with cursor pagination. Supports latest, hot, best, and top ordering.
// @Tags Tag
// @Produce json
// @Param tagName path string true "Normalized tag name"
// @Param sort query string false "Tag sort: hot, best, latest, top"
// @Param window query string false "Top window: 24h, 7d, 30d, all (allowed only when sort=top)"
// @Param limit query int false "Page size" minimum(1) maximum(1000)
// @Param cursor query string false "Opaque cursor returned by previous list response"
// @Success 200 {object} response.PostList
// @Failure 400 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /tags/{tagName}/posts [get]
func (h *HTTPHandler) handleTagPostsGet(c *gin.Context) {
	tagName := c.Param("tagName")
	if tagName == "" {
		badRequest(c, errors.New("tagName is required"))
		return
	}
	sortBy := strings.TrimSpace(c.Query("sort"))
	window := strings.TrimSpace(c.Query("window"))
	limit, cursor, ok := h.parseLimitCursor(c)
	if !ok {
		return
	}
	posts, err := h.postUseCase.GetPostsByTag(c.Request.Context(), tagName, sortBy, window, limit, cursor)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.PostListFromDTO(posts))
}

// handlePostSearchGet godoc
// @Summary List Feed Posts
// @Description Returns the global post feed ordered by hot, best, latest, or top with cursor pagination.
// @Tags Post
// @Produce json
// @Param sort query string false "Feed sort: hot, best, latest, top"
// @Param window query string false "Top window: 24h, 7d, 30d, all (allowed only when sort=top)"
// @Param limit query int false "Page size" minimum(1) maximum(1000)
// @Param cursor query string false "Opaque cursor returned by previous feed response"
// @Success 200 {object} response.PostList
// @Failure 400 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /posts/feed [get]
func (h *HTTPHandler) handlePostFeedGet(c *gin.Context) {
	sortBy := strings.TrimSpace(c.Query("sort"))
	window := strings.TrimSpace(c.Query("window"))
	limit, cursor, ok := h.parseLimitCursor(c)
	if !ok {
		return
	}
	posts, err := h.postUseCase.GetFeed(c.Request.Context(), sortBy, window, limit, cursor)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.PostListFromDTO(posts))
}

// handlePostSearchGet godoc
// @Summary Search Posts
// @Description Returns published posts matching title, content, and tag tokens with cursor pagination. Supports relevance, hot, latest, and top ordering.
// @Tags Post
// @Produce json
// @Param q query string true "Search query"
// @Param sort query string false "Search sort: relevance, hot, latest, top"
// @Param window query string false "Top window: 24h, 7d, 30d, all (allowed only when sort=top)"
// @Param limit query int false "Page size" minimum(1) maximum(1000)
// @Param cursor query string false "Opaque cursor returned by previous search response"
// @Success 200 {object} response.PostList
// @Failure 400 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /posts/search [get]
func (h *HTTPHandler) handlePostSearchGet(c *gin.Context) {
	query := strings.TrimSpace(c.Query("q"))
	sortBy := strings.TrimSpace(c.Query("sort"))
	window := strings.TrimSpace(c.Query("window"))
	if query == "" {
		badRequest(c, errors.New("query is required"))
		return
	}
	limit, cursor, ok := h.parseLimitCursor(c)
	if !ok {
		return
	}
	posts, err := h.postUseCase.SearchPosts(c.Request.Context(), query, sortBy, window, limit, cursor)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.PostListFromDTO(posts))
}

// handlePostDetailGet godoc
// @Summary Get Post Detail
// @Description Retrieves post detail by UUID.
// @Tags Post
// @Produce json
// @Param postUUID path string true "Post UUID" format(uuid)
// @Success 200 {object} response.PostDetail
// @Failure 400 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /posts/{postUUID} [get]
func (h *HTTPHandler) handlePostDetailGet(c *gin.Context) {
	postUUID, ok := parsePathUUID(c, "postUUID", "post")
	if !ok {
		return
	}

	post, err := h.postUseCase.GetPostDetail(c.Request.Context(), postUUID)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.PostDetailFromDTO(post))
}

// handlePostAttachmentsGet godoc
// @Summary List Post Attachments
// @Description Returns attachments for a published post.
// @Tags Attachment
// @Produce json
// @Param postUUID path string true "Post UUID" format(uuid)
// @Success 200 {object} attachmentListResponse
// @Failure 400 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /posts/{postUUID}/attachments [get]
func (h *HTTPHandler) handlePostAttachmentsGet(c *gin.Context) {
	postUUID, ok := parsePathUUID(c, "postUUID", "post")
	if !ok {
		return
	}
	items, err := h.attachmentUseCase.GetPostAttachments(c.Request.Context(), postUUID)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.JSON(http.StatusOK, attachmentListResponse{Attachments: response.AttachmentsFromDTO(items)})
}

// handlePostAttachmentsUpload godoc
// @Summary Upload Post Attachment
// @Description Uploads a file and creates attachment metadata for a post.
// @Tags Attachment
// @Accept mpfd
// @Produce json
// @Security BearerAuth
// @Param postUUID path string true "Post UUID" format(uuid)
// @Param file formData file true "Attachment file"
// @Success 201 {object} attachmentUploadResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /posts/{postUUID}/attachments/upload [post]
func (h *HTTPHandler) handlePostAttachmentsUpload(c *gin.Context) {
	postUUID, ok := parsePathUUID(c, "postUUID", "post")
	if !ok {
		return
	}
	userID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	if h.attachmentUploadMaxBytes > 0 {
		if c.Request.ContentLength > h.attachmentUploadMaxBytes+multipartRequestOverheadBytes {
			badRequest(c, errors.New("file too large"))
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.attachmentUploadMaxBytes+multipartRequestOverheadBytes)
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		if isMultipartBodyTooLarge(err) {
			badRequest(c, errors.New("file too large"))
			return
		}
		badRequest(c, errors.New("file is required"))
		return
	}
	if h.attachmentUploadMaxBytes > 0 && fileHeader.Size > h.attachmentUploadMaxBytes {
		badRequest(c, errors.New("file too large"))
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		writeUseCaseError(c, customerror.Wrap(customerror.ErrInternalServerError, "open upload file", err))
		return
	}
	defer file.Close()
	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" || contentType == "application/octet-stream" {
		if guessed := mime.TypeByExtension(filepath.Ext(fileHeader.Filename)); guessed != "" {
			contentType = guessed
		}
	}
	upload, err := h.attachmentUseCase.UploadPostAttachment(c.Request.Context(), postUUID, userID, fileHeader.Filename, contentType, file)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	previewURL := upload.PreviewURL
	if previewURL == "" {
		previewURL = fmt.Sprintf("/api/v1/posts/%s/attachments/%s/preview", postUUID, upload.UUID)
	}
	c.JSON(http.StatusCreated, attachmentUploadResponse{
		UUID:          upload.UUID,
		EmbedMarkdown: upload.EmbedMarkdown,
		PreviewURL:    previewURL,
	})
}

// handlePostAttachmentFileGet godoc
// @Summary Get Post Attachment File
// @Description Returns the stored file for an attachment of a published post.
// @Tags Attachment
// @Produce application/octet-stream
// @Param postUUID path string true "Post UUID" format(uuid)
// @Param attachmentUUID path string true "Attachment UUID" format(uuid)
// @Success 200 {file} file
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /posts/{postUUID}/attachments/{attachmentUUID}/file [get]
func (h *HTTPHandler) handlePostAttachmentFileGet(c *gin.Context) {
	postUUID, ok := parsePathUUID(c, "postUUID", "post")
	if !ok {
		return
	}
	attachmentUUID, ok := parsePathUUID(c, "attachmentUUID", "attachment")
	if !ok {
		return
	}
	file, err := h.attachmentUseCase.GetPostAttachmentFile(c.Request.Context(), postUUID, attachmentUUID)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	defer file.Content.Close()
	c.Header("Cache-Control", "no-store")
	if file.ETag != "" {
		c.Header("ETag", file.ETag)
		if c.GetHeader("If-None-Match") == file.ETag {
			c.Status(http.StatusNotModified)
			return
		}
	}
	c.Header("Content-Disposition", attachmentContentDisposition(file.FileName))
	c.DataFromReader(http.StatusOK, file.SizeBytes, file.ContentType, file.Content, nil)
}

// handlePostAttachmentPreviewGet godoc
// @Summary Preview Post Attachment File
// @Description Returns the stored file for an attachment of a draft or published post for the owner or admin.
// @Tags Attachment
// @Produce application/octet-stream
// @Security BearerAuth
// @Param postUUID path string true "Post UUID" format(uuid)
// @Param attachmentUUID path string true "Attachment UUID" format(uuid)
// @Success 200 {file} file
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /posts/{postUUID}/attachments/{attachmentUUID}/preview [get]
func (h *HTTPHandler) handlePostAttachmentPreviewGet(c *gin.Context) {
	postUUID, ok := parsePathUUID(c, "postUUID", "post")
	if !ok {
		return
	}
	attachmentUUID, ok := parsePathUUID(c, "attachmentUUID", "attachment")
	if !ok {
		return
	}
	userID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	file, err := h.attachmentUseCase.GetPostAttachmentPreviewFile(c.Request.Context(), postUUID, attachmentUUID, userID)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	defer file.Content.Close()
	c.Header("Cache-Control", "private, no-store")
	c.Header("Content-Disposition", attachmentContentDisposition(file.FileName))
	c.DataFromReader(http.StatusOK, file.SizeBytes, file.ContentType, file.Content, nil)
}

// handlePostAttachmentDelete godoc
// @Summary Delete Post Attachment
// @Description Deletes attachment metadata from a post.
// @Tags Attachment
// @Produce json
// @Security BearerAuth
// @Param postUUID path string true "Post UUID" format(uuid)
// @Param attachmentUUID path string true "Attachment UUID" format(uuid)
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /posts/{postUUID}/attachments/{attachmentUUID} [delete]
func (h *HTTPHandler) handlePostAttachmentDelete(c *gin.Context) {
	postUUID, ok := parsePathUUID(c, "postUUID", "post")
	if !ok {
		return
	}
	attachmentUUID, ok := parsePathUUID(c, "attachmentUUID", "attachment")
	if !ok {
		return
	}
	userID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	if err := h.attachmentUseCase.DeletePostAttachment(c.Request.Context(), postUUID, attachmentUUID, userID); err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// handlePostPublish godoc
// @Summary Publish Post
// @Description Publishes a draft post by UUID.
// @Tags Post
// @Produce json
// @Security BearerAuth
// @Param postUUID path string true "Post UUID" format(uuid)
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /posts/{postUUID}/publish [post]
func (h *HTTPHandler) handlePostPublish(c *gin.Context) {
	postUUID, ok := parsePathUUID(c, "postUUID", "post")
	if !ok {
		return
	}
	authorID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	if err := h.postUseCase.PublishPost(c.Request.Context(), postUUID, authorID); err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// handlePostDetailPut godoc
// @Summary Update Post
// @Description Updates a post by UUID.
// @Tags Post
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param postUUID path string true "Post UUID" format(uuid)
// @Param request body postRequest true "Update post payload"
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /posts/{postUUID} [put]
func (h *HTTPHandler) handlePostDetailPut(c *gin.Context) {
	postUUID, ok := parsePathUUID(c, "postUUID", "post")
	if !ok {
		return
	}
	authorID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	var req postRequest
	if err := h.decodeJSON(c, &req); err != nil {
		badRequest(c, err)
		return
	}
	if err := req.validate(); err != nil {
		badRequest(c, err)
		return
	}
	if err := h.postUseCase.UpdatePost(c.Request.Context(), postUUID, authorID, req.Title, req.Content, req.Tags); err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// handlePostDetailDelete godoc
// @Summary Delete Post
// @Description Deletes a post by UUID.
// @Tags Post
// @Produce json
// @Security BearerAuth
// @Param postUUID path string true "Post UUID" format(uuid)
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /posts/{postUUID} [delete]
func (h *HTTPHandler) handlePostDetailDelete(c *gin.Context) {
	postUUID, ok := parsePathUUID(c, "postUUID", "post")
	if !ok {
		return
	}
	authorID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	if err := h.postUseCase.DeletePost(c.Request.Context(), postUUID, authorID); err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// handlePostCommentsGet godoc
// @Summary List Comments by Post
// @Description Returns comments in post with cursor pagination.
// @Tags Comment
// @Produce json
// @Param postUUID path string true "Post UUID" format(uuid)
// @Param limit query int false "Page size" minimum(1) maximum(1000)
// @Param cursor query string false "Opaque cursor returned by previous list response"
// @Success 200 {object} response.CommentList
// @Failure 400 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /posts/{postUUID}/comments [get]
func (h *HTTPHandler) handlePostCommentsGet(c *gin.Context) {
	postUUID, ok := parsePathUUID(c, "postUUID", "post")
	if !ok {
		return
	}

	limit, cursor, ok := h.parseLimitCursor(c)
	if !ok {
		return
	}
	comments, err := h.commentUseCase.GetCommentsByPost(c.Request.Context(), postUUID, limit, cursor)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.CommentListFromDTO(comments))
}

// handlePostCommentsPost godoc
// @Summary Create Comment
// @Description Creates a comment on a post.
// @Tags Comment
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param postUUID path string true "Post UUID" format(uuid)
// @Param request body commentRequest true "Create comment payload"
// @Success 201 {object} uuidResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /posts/{postUUID}/comments [post]
func (h *HTTPHandler) handlePostCommentsPost(c *gin.Context) {
	postUUID, ok := parsePathUUID(c, "postUUID", "post")
	if !ok {
		return
	}
	authorID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	var req commentRequest
	if err := h.decodeJSON(c, &req); err != nil {
		badRequest(c, err)
		return
	}
	if err := req.validate(); err != nil {
		badRequest(c, err)
		return
	}
	id, err := h.commentUseCase.CreateComment(c.Request.Context(), req.Content, req.MentionedUsernames, authorID, postUUID, req.ParentUUID)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.JSON(http.StatusCreated, uuidResponse{UUID: id})
}

// handleCommentPut godoc
// @Summary Update Comment
// @Description Updates comment by UUID.
// @Tags Comment
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param commentUUID path string true "Comment UUID" format(uuid)
// @Param request body commentRequest true "Update comment payload"
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /comments/{commentUUID} [put]
func (h *HTTPHandler) handleCommentPut(c *gin.Context) {
	commentUUID, ok := parsePathUUID(c, "commentUUID", "comment")
	if !ok {
		return
	}

	authorID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	var req commentRequest
	if err := h.decodeJSON(c, &req); err != nil {
		badRequest(c, err)
		return
	}
	if err := req.validate(); err != nil {
		badRequest(c, err)
		return
	}
	if err := h.commentUseCase.UpdateComment(c.Request.Context(), commentUUID, authorID, req.Content); err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// handleCommentDelete godoc
// @Summary Delete Comment
// @Description Deletes comment by id.
// @Tags Comment
// @Produce json
// @Security BearerAuth
// @Param commentUUID path string true "Comment UUID" format(uuid)
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /comments/{commentUUID} [delete]
func (h *HTTPHandler) handleCommentDelete(c *gin.Context) {
	commentUUID, ok := parsePathUUID(c, "commentUUID", "comment")
	if !ok {
		return
	}
	authorID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	if err := h.commentUseCase.DeleteComment(c.Request.Context(), commentUUID, authorID); err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// handlePostReactions godoc
// @Summary List Reactions for Post
// @Description GET returns reactions for a post.
// @Tags Reaction
// @Accept json
// @Produce json
// @Param postUUID path string true "Post UUID" format(uuid)
// @Success 200 {array} response.Reaction
// @Failure 400 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /posts/{postUUID}/reactions [get]
func (h *HTTPHandler) handlePostReactions(c *gin.Context) {
	postUUID, ok := parsePathUUID(c, "postUUID", "post")
	if !ok {
		return
	}
	h.handleReactionsByTarget(c, postUUID, model.ReactionTargetPost)
}

// handleCommentReactions godoc
// @Summary List Reactions for Comment
// @Description GET returns reactions for a comment.
// @Tags Reaction
// @Accept json
// @Produce json
// @Param commentUUID path string true "Comment UUID" format(uuid)
// @Success 200 {array} response.Reaction
// @Failure 400 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /comments/{commentUUID}/reactions [get]
func (h *HTTPHandler) handleCommentReactions(c *gin.Context) {
	commentUUID, ok := parsePathUUID(c, "commentUUID", "comment")
	if !ok {
		return
	}
	h.handleReactionsByTarget(c, commentUUID, model.ReactionTargetComment)
}

// handleMyPostReactionPut godoc
// @ID setMyPostReaction
// @Summary Set My Reaction for Post
// @Description PUT creates or updates the current user's reaction on a post.
// @Tags Reaction
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param postUUID path string true "Post UUID" format(uuid)
// @Param request body reactionRequest true "Set reaction payload"
// @Success 201
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /posts/{postUUID}/reactions/me [put]
func (h *HTTPHandler) handleMyPostReactionPut(c *gin.Context) {
	postUUID, ok := parsePathUUID(c, "postUUID", "post")
	if !ok {
		return
	}
	h.handleMyReactionPut(c, postUUID, model.ReactionTargetPost)
}

// handleMyPostReactionDelete godoc
// @ID deleteMyPostReaction
// @Summary Delete My Reaction for Post
// @Description DELETE removes the current user's reaction on a post.
// @Tags Reaction
// @Produce json
// @Security BearerAuth
// @Param postUUID path string true "Post UUID" format(uuid)
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /posts/{postUUID}/reactions/me [delete]
func (h *HTTPHandler) handleMyPostReactionDelete(c *gin.Context) {
	postUUID, ok := parsePathUUID(c, "postUUID", "post")
	if !ok {
		return
	}
	h.handleMyReactionDelete(c, postUUID, model.ReactionTargetPost)
}

// handleMyCommentReactionPut godoc
// @ID setMyCommentReaction
// @Summary Set My Reaction for Comment
// @Description PUT creates or updates the current user's reaction on a comment.
// @Tags Reaction
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param commentUUID path string true "Comment UUID" format(uuid)
// @Param request body reactionRequest true "Set reaction payload"
// @Success 201
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /comments/{commentUUID}/reactions/me [put]
func (h *HTTPHandler) handleMyCommentReactionPut(c *gin.Context) {
	commentUUID, ok := parsePathUUID(c, "commentUUID", "comment")
	if !ok {
		return
	}
	h.handleMyReactionPut(c, commentUUID, model.ReactionTargetComment)
}

// handleMyCommentReactionDelete godoc
// @ID deleteMyCommentReaction
// @Summary Delete My Reaction for Comment
// @Description DELETE removes the current user's reaction on a comment.
// @Tags Reaction
// @Produce json
// @Security BearerAuth
// @Param commentUUID path string true "Comment UUID" format(uuid)
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /comments/{commentUUID}/reactions/me [delete]
func (h *HTTPHandler) handleMyCommentReactionDelete(c *gin.Context) {
	commentUUID, ok := parsePathUUID(c, "commentUUID", "comment")
	if !ok {
		return
	}
	h.handleMyReactionDelete(c, commentUUID, model.ReactionTargetComment)
}

func (h *HTTPHandler) handleReactionsByTarget(c *gin.Context, targetUUID string, targetType model.ReactionTargetType) {
	reactions, err := h.reactionUseCase.GetReactionsByTarget(c.Request.Context(), targetUUID, targetType)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.ReactionsFromDTO(reactions))
}

func (h *HTTPHandler) handleMyReactionPut(c *gin.Context, targetUUID string, targetType model.ReactionTargetType) {
	userID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	var req reactionRequest
	if err := h.decodeJSON(c, &req); err != nil {
		badRequest(c, err)
		return
	}
	reactionType, err := req.parseType()
	if err != nil {
		badRequest(c, err)
		return
	}
	created, err := h.reactionUseCase.SetReaction(c.Request.Context(), userID, targetUUID, targetType, reactionType)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	if created {
		c.Status(http.StatusCreated)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *HTTPHandler) handleMyReactionDelete(c *gin.Context, targetUUID string, targetType model.ReactionTargetType) {
	userID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	if err := h.reactionUseCase.DeleteReaction(c.Request.Context(), userID, targetUUID, targetType); err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func reportResponsesFromModel(items []model.Report) []reportResponse {
	out := make([]reportResponse, 0, len(items))
	for _, item := range items {
		out = append(out, reportResponse{
			ID:             item.ID,
			TargetType:     item.TargetType,
			TargetUUID:     item.TargetUUID,
			ReporterUUID:   item.ReporterUUID,
			ReasonCode:     item.ReasonCode,
			ReasonDetail:   item.ReasonDetail,
			Status:         item.Status,
			ResolutionNote: item.ResolutionNote,
			ResolvedByUUID: item.ResolvedByUUID,
			ResolvedAt:     item.ResolvedAt,
			CreatedAt:      item.CreatedAt,
			UpdatedAt:      item.UpdatedAt,
		})
	}
	return out
}

func outboxDeadResponsesFromModel(items []model.OutboxDeadMessage) []outboxDeadMessageResponse {
	out := make([]outboxDeadMessageResponse, 0, len(items))
	for _, item := range items {
		out = append(out, outboxDeadMessageResponse{
			ID:            item.ID,
			EventName:     item.EventName,
			AttemptCount:  item.AttemptCount,
			LastError:     item.LastError,
			OccurredAt:    item.OccurredAt,
			NextAttemptAt: item.NextAttemptAt,
		})
	}
	return out
}

func writeUseCaseError(c *gin.Context, err error) {
	publicErr := customerror.Public(err)
	writeHTTPError(loggerFromContext(c), c, statusForError(err), publicErr)
}

func writeHTTPError(logger *slog.Logger, c *gin.Context, status int, err error) {
	publicErr := customerror.Public(err)
	logAttrs := []any{
		"method", c.Request.Method,
		"path", c.FullPath(),
		"status", status,
		"error", err,
		"public_error", publicErr.Error(),
	}
	if c.Request.URL != nil {
		logAttrs = append(logAttrs, "request_uri", c.Request.URL.RequestURI())
	}
	if userID, ok := middleware.UserID(c); ok {
		logAttrs = append(logAttrs, "user_id", userID)
	}
	if status >= http.StatusInternalServerError {
		logger.Error("request failed", logAttrs...)
	} else {
		logger.Warn("request failed", logAttrs...)
	}
	c.AbortWithStatusJSON(status, errorResponse{Error: publicErr.Error()})
}

func loggerFromContext(c *gin.Context) *slog.Logger {
	if c != nil {
		if v, ok := c.Get(httpLoggerContextKey); ok {
			if logger, ok := v.(*slog.Logger); ok && logger != nil {
				return logger
			}
		}
	}
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func statusForError(err error) int {
	switch {
	case errors.Is(err, customerror.ErrTooManyRequests):
		return http.StatusTooManyRequests
	case errors.Is(err, customerror.ErrUserAlreadyExists):
		return http.StatusConflict
	case errors.Is(err, customerror.ErrInvalidInput):
		return http.StatusBadRequest
	case errors.Is(err, customerror.ErrUserNotFound):
		return http.StatusNotFound
	case errors.Is(err, customerror.ErrBoardNotFound):
		return http.StatusNotFound
	case errors.Is(err, customerror.ErrBoardNotEmpty):
		return http.StatusConflict
	case errors.Is(err, customerror.ErrPostNotFound):
		return http.StatusNotFound
	case errors.Is(err, customerror.ErrTagNotFound):
		return http.StatusNotFound
	case errors.Is(err, customerror.ErrAttachmentNotFound):
		return http.StatusNotFound
	case errors.Is(err, customerror.ErrCommentNotFound):
		return http.StatusNotFound
	case errors.Is(err, customerror.ErrReactionNotFound):
		return http.StatusNotFound
	case errors.Is(err, customerror.ErrReportNotFound):
		return http.StatusNotFound
	case errors.Is(err, customerror.ErrReportAlreadyExists):
		return http.StatusConflict
	case errors.Is(err, customerror.ErrInvalidCredential):
		return http.StatusUnauthorized
	case errors.Is(err, customerror.ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, customerror.ErrMissingAuthHeader):
		return http.StatusUnauthorized
	case errors.Is(err, customerror.ErrInvalidToken):
		return http.StatusUnauthorized
	case errors.Is(err, customerror.ErrEmailVerificationRequired):
		return http.StatusForbidden
	case errors.Is(err, customerror.ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, customerror.ErrUserSuspended):
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}

func (h *HTTPHandler) rateLimitGinMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h == nil || !h.rateLimitEnabled || h.rateLimiter == nil {
			c.Next()
			return
		}
		if !isWriteMethod(c.Request.Method) && !isReadMethod(c.Request.Method) {
			c.Next()
			return
		}
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		key := c.Request.Method + ":" + path + ":" + c.ClientIP()
		limit := h.rateLimitWriteRequests
		if isReadMethod(c.Request.Method) {
			limit = h.rateLimitReadRequests
		}
		allowed, err := h.rateLimiter.Allow(c.Request.Context(), key, limit, h.rateLimitWindow)
		if err != nil {
			writeHTTPError(h.logger, c, http.StatusInternalServerError, customerror.Wrap(customerror.ErrInternalServerError, "rate limit", err))
			return
		}
		if !allowed {
			writeHTTPError(h.logger, c, http.StatusTooManyRequests, customerror.ErrTooManyRequests)
			return
		}
		c.Next()
	}
}

func isWriteMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch:
		return true
	default:
		return false
	}
}

func isReadMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

func passwordResetRateLimitKey(clientIP, email string) string {
	return "password-reset-request:" + clientIP + ":" + normalizePasswordResetEmail(email)
}

func normalizePasswordResetEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func hashEmailForAudit(email string) string {
	sum := sha256.Sum256([]byte(normalizePasswordResetEmail(email)))
	return fmt.Sprintf("%x", sum[:])
}

func (h *HTTPHandler) parseLimitCursor(c *gin.Context) (int, string, bool) {
	limitStr := c.Query("limit")
	cursor := strings.TrimSpace(c.Query("cursor"))

	limit := h.defaultPageLimit

	if limitStr != "" {
		v, err := strconv.Atoi(limitStr)
		if err != nil || v < 1 || v > maxPageLimit {
			badRequest(c, errors.New("invalid limit"))
			return 0, "", false
		}
		limit = v
	}
	return limit, cursor, true
}

func (h *HTTPHandler) parseLimitLastID(c *gin.Context) (int, int64, bool) {
	limitStr := c.Query("limit")
	lastIDStr := c.Query("last_id")

	limit := h.defaultPageLimit
	var lastID int64

	if limitStr != "" {
		v, err := strconv.Atoi(limitStr)
		if err != nil || v < 1 || v > maxPageLimit {
			badRequest(c, errors.New("invalid limit"))
			return 0, 0, false
		}
		limit = v
	}

	if lastIDStr != "" {
		v, err := strconv.ParseInt(lastIDStr, 10, 64)
		if err != nil || v < 0 {
			badRequest(c, errors.New("invalid last_id"))
			return 0, 0, false
		}
		lastID = v
	}

	return limit, lastID, true
}

func (h *HTTPHandler) parseLimitLastIDString(c *gin.Context) (int, string, bool) {
	limitStr := c.Query("limit")
	lastID := strings.TrimSpace(c.Query("last_id"))

	limit := h.defaultPageLimit
	if limitStr != "" {
		v, err := strconv.Atoi(limitStr)
		if err != nil || v < 1 || v > maxPageLimit {
			badRequest(c, errors.New("invalid limit"))
			return 0, "", false
		}
		limit = v
	}
	return limit, lastID, true
}

func parseInt64(raw string) (int64, error) {
	if raw == "" {
		return 0, errors.New("value is required")
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, err
	}
	if v < 1 {
		return 0, errors.New("value must be >= 1")
	}
	return v, nil
}

func parsePathID(c *gin.Context, paramName, resourceName string) (int64, bool) {
	id, err := parseInt64(c.Param(paramName))
	if err != nil {
		badRequest(c, errors.New("invalid "+resourceName+" id"))
		return 0, false
	}
	return id, true
}

func parsePathUUID(c *gin.Context, paramName, resourceName string) (string, bool) {
	raw := strings.TrimSpace(c.Param(paramName))
	if _, err := uuid.Parse(raw); err != nil {
		badRequest(c, errors.New("invalid "+resourceName+" uuid"))
		return "", false
	}
	return raw, true
}

func (h *HTTPHandler) decodeJSON(c *gin.Context, dst any) error {
	defer c.Request.Body.Close()
	if h != nil && h.maxJSONBodyBytes > 0 {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.maxJSONBodyBytes)
	}
	dec := json.NewDecoder(c.Request.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return errors.New("request body too large")
		}
		return err
	}
	if err := dec.Decode(&struct{}{}); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return errors.New("request body too large")
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	return errors.New("invalid JSON body")
}

func badRequest(c *gin.Context, err error) {
	c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
}

func attachmentContentDisposition(fileName string) string {
	value := mime.FormatMediaType("inline", map[string]string{"filename": fileName})
	if value == "" {
		return "inline"
	}
	return value
}

func isMultipartBodyTooLarge(err error) bool {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "too large")
}
