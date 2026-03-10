package delivery

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/delivery/middleware"
	"github.com/hoonzinope/go-comu-bin/internal/delivery/response"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

const multipartRequestOverheadBytes int64 = 1 << 20
const httpLoggerContextKey = "http_logger"

type noopLogger struct{}

func (noopLogger) Warn(string, ...any)  {}
func (noopLogger) Error(string, ...any) {}

type HTTPHandler struct {
	sessionUseCase           port.SessionUseCase
	userUseCase              port.UserUseCase
	accountUseCase           port.AccountUseCase
	boardUseCase             port.BoardUseCase
	postUseCase              port.PostUseCase
	commentUseCase           port.CommentUseCase
	reactionUseCase          port.ReactionUseCase
	attachmentUseCase        port.AttachmentUseCase
	attachmentUploadMaxBytes int64
	logger                   port.Logger
	authGinMiddleware        gin.HandlerFunc
}

type HTTPDependencies struct {
	SessionUseCase           port.SessionUseCase
	UserUseCase              port.UserUseCase
	AccountUseCase           port.AccountUseCase
	BoardUseCase             port.BoardUseCase
	PostUseCase              port.PostUseCase
	CommentUseCase           port.CommentUseCase
	ReactionUseCase          port.ReactionUseCase
	AttachmentUseCase        port.AttachmentUseCase
	AttachmentUploadMaxBytes int64
	Logger                   port.Logger
}

func NewHTTPHandler(deps HTTPDependencies) *HTTPHandler {
	logger := deps.Logger
	if logger == nil {
		logger = noopLogger{}
	}
	handler := &HTTPHandler{
		sessionUseCase:           deps.SessionUseCase,
		userUseCase:              deps.UserUseCase,
		accountUseCase:           deps.AccountUseCase,
		boardUseCase:             deps.BoardUseCase,
		postUseCase:              deps.PostUseCase,
		commentUseCase:           deps.CommentUseCase,
		reactionUseCase:          deps.ReactionUseCase,
		attachmentUseCase:        deps.AttachmentUseCase,
		attachmentUploadMaxBytes: deps.AttachmentUploadMaxBytes,
		logger:                   logger,
	}
	handler.authGinMiddleware = middleware.AuthWithSession(deps.SessionUseCase, func(c *gin.Context, status int, err error) {
		writeHTTPError(handler.logger, c, status, err)
	})
	return handler
}

func (h *HTTPHandler) RegisterRoutes(r *gin.Engine) {
	r.Use(func(c *gin.Context) {
		c.Set(httpLoggerContextKey, h.logger)
		c.Next()
	})
	r.HandleMethodNotAllowed = true
	r.NoMethod(func(c *gin.Context) {
		writeHTTPError(h.logger, c, http.StatusMethodNotAllowed, customError.ErrMethodNotAllowed)
	})
	r.NoRoute(func(c *gin.Context) {
		writeHTTPError(h.logger, c, http.StatusNotFound, customError.ErrNotFound)
	})
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	v1 := r.Group("/api/v1")
	v1.POST("/signup", h.handleUserSignUp)
	v1.POST("/auth/login", h.handleUserLogin)
	v1.POST("/auth/logout", h.authGinMiddleware, h.handleUserLogout)
	v1.DELETE("/users/me", h.authGinMiddleware, h.handleUserDeleteMe)
	v1.GET("/users/:userUUID/suspension", h.authGinMiddleware, h.handleUserSuspensionGet)
	v1.PUT("/users/:userUUID/suspension", h.authGinMiddleware, h.handleUserSuspend)
	v1.DELETE("/users/:userUUID/suspension", h.authGinMiddleware, h.handleUserUnsuspend)

	v1.GET("/boards", h.handleBoardsGet)
	v1.POST("/boards", h.authGinMiddleware, h.handleBoardsPost)
	v1.PUT("/boards/:boardID", h.authGinMiddleware, h.handleBoardPut)
	v1.DELETE("/boards/:boardID", h.authGinMiddleware, h.handleBoardDelete)

	v1.GET("/boards/:boardID/posts", h.handleBoardPostsGet)
	v1.POST("/boards/:boardID/posts", h.authGinMiddleware, h.handleBoardPostsPost)
	v1.POST("/boards/:boardID/posts/drafts", h.authGinMiddleware, h.handleBoardDraftPostsPost)
	v1.GET("/tags/:tagName/posts", h.handleTagPostsGet)

	v1.GET("/posts/:postID", h.handlePostDetailGet)
	v1.POST("/posts/:postID/publish", h.authGinMiddleware, h.handlePostPublish)
	v1.GET("/posts/:postID/attachments", h.handlePostAttachmentsGet)
	v1.GET("/posts/:postID/attachments/:attachmentID/file", h.handlePostAttachmentFileGet)
	v1.GET("/posts/:postID/attachments/:attachmentID/preview", h.authGinMiddleware, h.handlePostAttachmentPreviewGet)
	v1.POST("/posts/:postID/attachments/upload", h.authGinMiddleware, h.handlePostAttachmentsUpload)
	v1.DELETE("/posts/:postID/attachments/:attachmentID", h.authGinMiddleware, h.handlePostAttachmentDelete)
	v1.PUT("/posts/:postID", h.authGinMiddleware, h.handlePostDetailPut)
	v1.DELETE("/posts/:postID", h.authGinMiddleware, h.handlePostDetailDelete)

	v1.GET("/posts/:postID/comments", h.handlePostCommentsGet)
	v1.POST("/posts/:postID/comments", h.authGinMiddleware, h.handlePostCommentsPost)
	v1.GET("/posts/:postID/reactions", h.handlePostReactions)
	v1.PUT("/posts/:postID/reactions/me", h.authGinMiddleware, h.handleMyPostReactionPut)
	v1.DELETE("/posts/:postID/reactions/me", h.authGinMiddleware, h.handleMyPostReactionDelete)

	v1.PUT("/comments/:commentID", h.authGinMiddleware, h.handleCommentPut)
	v1.DELETE("/comments/:commentID", h.authGinMiddleware, h.handleCommentDelete)
	v1.GET("/comments/:commentID/reactions", h.handleCommentReactions)
	v1.PUT("/comments/:commentID/reactions/me", h.authGinMiddleware, h.handleMyCommentReactionPut)
	v1.DELETE("/comments/:commentID/reactions/me", h.authGinMiddleware, h.handleMyCommentReactionDelete)
}

func NewHTTPServer(addr string, deps HTTPDependencies) *http.Server {
	r := gin.New()
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
		c.JSON(http.StatusUnauthorized, errorResponse{Error: customError.ErrUnauthorized.Error()})
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
// @Param request body userCredentialRequest true "Sign up payload"
// @Success 201 {object} signUpResponse
// @Failure 400 {object} errorResponse
// @Failure 409 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /signup [post]
func (h *HTTPHandler) handleUserSignUp(c *gin.Context) {
	var req userCredentialRequest
	if err := decodeJSON(c, &req); err != nil {
		badRequest(c, err)
		return
	}
	if err := req.validate(); err != nil {
		badRequest(c, err)
		return
	}
	if _, err := h.userUseCase.SignUp(req.Username, req.Password); err != nil {
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
	if err := decodeJSON(c, &req); err != nil {
		badRequest(c, err)
		return
	}
	if err := req.validate(); err != nil {
		badRequest(c, err)
		return
	}
	token, err := h.sessionUseCase.Login(req.Username, req.Password)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.Header("Authorization", "Bearer "+token)
	c.JSON(http.StatusOK, loginResponse{Login: "ok"})
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
		c.JSON(http.StatusUnauthorized, errorResponse{Error: customError.ErrUnauthorized.Error()})
		return
	}
	if err := h.sessionUseCase.Logout(token); err != nil {
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
	if err := decodeJSON(c, &req); err != nil {
		badRequest(c, err)
		return
	}
	if err := req.validate(); err != nil {
		badRequest(c, err)
		return
	}
	if err := h.accountUseCase.DeleteMyAccount(userID, req.Password); err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
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
	targetUserUUID := strings.TrimSpace(c.Param("userUUID"))
	if targetUserUUID == "" {
		badRequest(c, errors.New("invalid user uuid"))
		return
	}
	adminID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	view, err := h.userUseCase.GetUserSuspension(adminID, targetUserUUID)
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
	targetUserUUID := strings.TrimSpace(c.Param("userUUID"))
	if targetUserUUID == "" {
		badRequest(c, errors.New("invalid user uuid"))
		return
	}
	adminID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	var req userSuspensionRequest
	if err := decodeJSON(c, &req); err != nil {
		badRequest(c, err)
		return
	}
	reason, duration, err := req.parse()
	if err != nil {
		badRequest(c, err)
		return
	}
	if err := h.userUseCase.SuspendUser(adminID, targetUserUUID, reason, duration); err != nil {
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
	targetUserUUID := strings.TrimSpace(c.Param("userUUID"))
	if targetUserUUID == "" {
		badRequest(c, errors.New("invalid user uuid"))
		return
	}
	adminID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	if err := h.userUseCase.UnsuspendUser(adminID, targetUserUUID); err != nil {
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
// @Param limit query int false "Page size" minimum(1)
// @Param last_id query int false "Cursor id, fetch items with id < last_id" minimum(0)
// @Success 200 {object} response.BoardList
// @Failure 400 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /boards [get]
func (h *HTTPHandler) handleBoardsGet(c *gin.Context) {
	limit, lastID, ok := parseLimitLastID(c)
	if !ok {
		return
	}
	boards, err := h.boardUseCase.GetBoards(limit, lastID)
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
// @Success 201 {object} idResponse
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
	if err := decodeJSON(c, &req); err != nil {
		badRequest(c, err)
		return
	}
	if err := req.validate(); err != nil {
		badRequest(c, err)
		return
	}
	id, err := h.boardUseCase.CreateBoard(userID, req.Name, req.Description)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.JSON(http.StatusCreated, idResponse{ID: id})
}

// handleBoardPut godoc
// @Summary Update Board
// @Description Updates a board by id (admin only).
// @Tags Board
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param boardID path int true "Board ID"
// @Param request body boardRequest true "Update board payload"
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /boards/{boardID} [put]
func (h *HTTPHandler) handleBoardPut(c *gin.Context) {
	boardID, ok := parsePathID(c, "boardID", "board")
	if !ok {
		return
	}

	var req boardRequest
	if err := decodeJSON(c, &req); err != nil {
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
	if err := h.boardUseCase.UpdateBoard(boardID, userID, req.Name, req.Description); err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// handleBoardDelete godoc
// @Summary Delete Board
// @Description Deletes a board by id (admin only).
// @Tags Board
// @Produce json
// @Security BearerAuth
// @Param boardID path int true "Board ID"
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /boards/{boardID} [delete]
func (h *HTTPHandler) handleBoardDelete(c *gin.Context) {
	boardID, ok := parsePathID(c, "boardID", "board")
	if !ok {
		return
	}
	userID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	if err := h.boardUseCase.DeleteBoard(boardID, userID); err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// handleBoardPostsGet godoc
// @Summary List Posts by Board
// @Description Returns posts in board with cursor pagination.
// @Tags Post
// @Produce json
// @Param boardID path int true "Board ID"
// @Param limit query int false "Page size" minimum(1)
// @Param last_id query int false "Cursor id, fetch items with id < last_id" minimum(0)
// @Success 200 {object} response.PostList
// @Failure 400 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /boards/{boardID}/posts [get]
func (h *HTTPHandler) handleBoardPostsGet(c *gin.Context) {
	boardID, ok := parsePathID(c, "boardID", "board")
	if !ok {
		return
	}

	limit, lastID, ok := parseLimitLastID(c)
	if !ok {
		return
	}
	posts, err := h.postUseCase.GetPostsList(boardID, limit, lastID)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.PostListFromDTO(posts))
}

// handleBoardPostsPost godoc
// @Summary Create Post
// @Description Creates a post in board.
// @Tags Post
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param boardID path int true "Board ID"
// @Param request body postRequest true "Create post payload"
// @Success 201 {object} idResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /boards/{boardID}/posts [post]
func (h *HTTPHandler) handleBoardPostsPost(c *gin.Context) {
	boardID, ok := parsePathID(c, "boardID", "board")
	if !ok {
		return
	}
	authorID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	var req postRequest
	if err := decodeJSON(c, &req); err != nil {
		badRequest(c, err)
		return
	}
	if err := req.validate(); err != nil {
		badRequest(c, err)
		return
	}
	postID, err := h.postUseCase.CreatePost(req.Title, req.Content, req.Tags, authorID, boardID)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.JSON(http.StatusCreated, idResponse{ID: postID})
}

// handleBoardDraftPostsPost godoc
// @Summary Create Draft Post
// @Description Creates a draft post in board.
// @Tags Post
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param boardID path int true "Board ID"
// @Param request body postRequest true "Create draft post payload"
// @Success 201 {object} idResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /boards/{boardID}/posts/drafts [post]
func (h *HTTPHandler) handleBoardDraftPostsPost(c *gin.Context) {
	boardID, ok := parsePathID(c, "boardID", "board")
	if !ok {
		return
	}
	authorID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	var req postRequest
	if err := decodeJSON(c, &req); err != nil {
		badRequest(c, err)
		return
	}
	if err := req.validate(); err != nil {
		badRequest(c, err)
		return
	}
	postID, err := h.postUseCase.CreateDraftPost(req.Title, req.Content, req.Tags, authorID, boardID)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.JSON(http.StatusCreated, idResponse{ID: postID})
}

// handleTagPostsGet godoc
// @Summary List Posts by Tag
// @Description Returns posts connected to a tag with cursor pagination.
// @Tags Tag
// @Produce json
// @Param tagName path string true "Normalized tag name"
// @Param limit query int false "Page size" minimum(1)
// @Param last_id query int false "Cursor id, fetch items with id < last_id" minimum(0)
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
	limit, lastID, ok := parseLimitLastID(c)
	if !ok {
		return
	}
	posts, err := h.postUseCase.GetPostsByTag(tagName, limit, lastID)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.PostListFromDTO(posts))
}

// handlePostDetailGet godoc
// @Summary Get Post Detail
// @Description Retrieves post detail by id.
// @Tags Post
// @Produce json
// @Param postID path int true "Post ID"
// @Success 200 {object} response.PostDetail
// @Failure 400 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /posts/{postID} [get]
func (h *HTTPHandler) handlePostDetailGet(c *gin.Context) {
	postID, ok := parsePathID(c, "postID", "post")
	if !ok {
		return
	}

	post, err := h.postUseCase.GetPostDetail(postID)
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
// @Param postID path int true "Post ID"
// @Success 200 {object} attachmentListResponse
// @Failure 400 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /posts/{postID}/attachments [get]
func (h *HTTPHandler) handlePostAttachmentsGet(c *gin.Context) {
	postID, ok := parsePathID(c, "postID", "post")
	if !ok {
		return
	}
	items, err := h.attachmentUseCase.GetPostAttachments(postID)
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
// @Param postID path int true "Post ID"
// @Param file formData file true "Attachment file"
// @Success 201 {object} attachmentUploadResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /posts/{postID}/attachments/upload [post]
func (h *HTTPHandler) handlePostAttachmentsUpload(c *gin.Context) {
	postID, ok := parsePathID(c, "postID", "post")
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
		writeUseCaseError(c, customError.Wrap(customError.ErrInternalServerError, "open upload file", err))
		return
	}
	defer file.Close()
	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" || contentType == "application/octet-stream" {
		if guessed := mime.TypeByExtension(filepath.Ext(fileHeader.Filename)); guessed != "" {
			contentType = guessed
		}
	}
	upload, err := h.attachmentUseCase.UploadPostAttachment(postID, userID, fileHeader.Filename, contentType, file)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	previewURL := upload.PreviewURL
	if previewURL == "" {
		previewURL = fmt.Sprintf("/api/v1/posts/%d/attachments/%d/preview", postID, upload.ID)
	}
	c.JSON(http.StatusCreated, attachmentUploadResponse{
		ID:            upload.ID,
		EmbedMarkdown: upload.EmbedMarkdown,
		PreviewURL:    previewURL,
	})
}

// handlePostAttachmentFileGet godoc
// @Summary Get Post Attachment File
// @Description Returns the stored file for an attachment of a published post.
// @Tags Attachment
// @Produce application/octet-stream
// @Param postID path int true "Post ID"
// @Param attachmentID path int true "Attachment ID"
// @Success 200 {file} file
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /posts/{postID}/attachments/{attachmentID}/file [get]
func (h *HTTPHandler) handlePostAttachmentFileGet(c *gin.Context) {
	postID, ok := parsePathID(c, "postID", "post")
	if !ok {
		return
	}
	attachmentID, ok := parsePathID(c, "attachmentID", "attachment")
	if !ok {
		return
	}
	file, err := h.attachmentUseCase.GetPostAttachmentFile(postID, attachmentID)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	defer file.Content.Close()
	c.Header("Cache-Control", "public, max-age=300")
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
// @Param postID path int true "Post ID"
// @Param attachmentID path int true "Attachment ID"
// @Success 200 {file} file
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /posts/{postID}/attachments/{attachmentID}/preview [get]
func (h *HTTPHandler) handlePostAttachmentPreviewGet(c *gin.Context) {
	postID, ok := parsePathID(c, "postID", "post")
	if !ok {
		return
	}
	attachmentID, ok := parsePathID(c, "attachmentID", "attachment")
	if !ok {
		return
	}
	userID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	file, err := h.attachmentUseCase.GetPostAttachmentPreviewFile(postID, attachmentID, userID)
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
// @Param postID path int true "Post ID"
// @Param attachmentID path int true "Attachment ID"
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /posts/{postID}/attachments/{attachmentID} [delete]
func (h *HTTPHandler) handlePostAttachmentDelete(c *gin.Context) {
	postID, ok := parsePathID(c, "postID", "post")
	if !ok {
		return
	}
	attachmentID, ok := parsePathID(c, "attachmentID", "attachment")
	if !ok {
		return
	}
	userID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	if err := h.attachmentUseCase.DeletePostAttachment(postID, attachmentID, userID); err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// handlePostPublish godoc
// @Summary Publish Post
// @Description Publishes a draft post by id.
// @Tags Post
// @Produce json
// @Security BearerAuth
// @Param postID path int true "Post ID"
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /posts/{postID}/publish [post]
func (h *HTTPHandler) handlePostPublish(c *gin.Context) {
	postID, ok := parsePathID(c, "postID", "post")
	if !ok {
		return
	}
	authorID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	if err := h.postUseCase.PublishPost(postID, authorID); err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// handlePostDetailPut godoc
// @Summary Update Post
// @Description Updates a post by id.
// @Tags Post
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param postID path int true "Post ID"
// @Param request body postRequest true "Update post payload"
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /posts/{postID} [put]
func (h *HTTPHandler) handlePostDetailPut(c *gin.Context) {
	postID, ok := parsePathID(c, "postID", "post")
	if !ok {
		return
	}
	authorID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	var req postRequest
	if err := decodeJSON(c, &req); err != nil {
		badRequest(c, err)
		return
	}
	if err := req.validate(); err != nil {
		badRequest(c, err)
		return
	}
	if err := h.postUseCase.UpdatePost(postID, authorID, req.Title, req.Content, req.Tags); err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// handlePostDetailDelete godoc
// @Summary Delete Post
// @Description Deletes a post by id.
// @Tags Post
// @Produce json
// @Security BearerAuth
// @Param postID path int true "Post ID"
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /posts/{postID} [delete]
func (h *HTTPHandler) handlePostDetailDelete(c *gin.Context) {
	postID, ok := parsePathID(c, "postID", "post")
	if !ok {
		return
	}
	authorID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	if err := h.postUseCase.DeletePost(postID, authorID); err != nil {
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
// @Param postID path int true "Post ID"
// @Param limit query int false "Page size" minimum(1)
// @Param last_id query int false "Cursor id, fetch items with id < last_id" minimum(0)
// @Success 200 {object} response.CommentList
// @Failure 400 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /posts/{postID}/comments [get]
func (h *HTTPHandler) handlePostCommentsGet(c *gin.Context) {
	postID, ok := parsePathID(c, "postID", "post")
	if !ok {
		return
	}

	limit, lastID, ok := parseLimitLastID(c)
	if !ok {
		return
	}
	comments, err := h.commentUseCase.GetCommentsByPost(postID, limit, lastID)
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
// @Param postID path int true "Post ID"
// @Param request body commentRequest true "Create comment payload"
// @Success 201 {object} idResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /posts/{postID}/comments [post]
func (h *HTTPHandler) handlePostCommentsPost(c *gin.Context) {
	postID, ok := parsePathID(c, "postID", "post")
	if !ok {
		return
	}
	authorID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	var req commentRequest
	if err := decodeJSON(c, &req); err != nil {
		badRequest(c, err)
		return
	}
	if err := req.validate(); err != nil {
		badRequest(c, err)
		return
	}
	id, err := h.commentUseCase.CreateComment(req.Content, authorID, postID, req.ParentID)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.JSON(http.StatusCreated, idResponse{ID: id})
}

// handleCommentPut godoc
// @Summary Update Comment
// @Description Updates comment by id.
// @Tags Comment
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param commentID path int true "Comment ID"
// @Param request body commentRequest true "Update comment payload"
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /comments/{commentID} [put]
func (h *HTTPHandler) handleCommentPut(c *gin.Context) {
	commentID, ok := parsePathID(c, "commentID", "comment")
	if !ok {
		return
	}

	authorID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	var req commentRequest
	if err := decodeJSON(c, &req); err != nil {
		badRequest(c, err)
		return
	}
	if err := req.validate(); err != nil {
		badRequest(c, err)
		return
	}
	if err := h.commentUseCase.UpdateComment(commentID, authorID, req.Content); err != nil {
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
// @Param commentID path int true "Comment ID"
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /comments/{commentID} [delete]
func (h *HTTPHandler) handleCommentDelete(c *gin.Context) {
	commentID, ok := parsePathID(c, "commentID", "comment")
	if !ok {
		return
	}
	authorID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	if err := h.commentUseCase.DeleteComment(commentID, authorID); err != nil {
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
// @Param postID path int true "Post ID"
// @Success 200 {array} response.Reaction
// @Failure 400 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /posts/{postID}/reactions [get]
func (h *HTTPHandler) handlePostReactions(c *gin.Context) {
	postID, ok := parsePathID(c, "postID", "post")
	if !ok {
		return
	}
	h.handleReactionsByTarget(c, postID, entity.ReactionTargetPost)
}

// handleCommentReactions godoc
// @Summary List Reactions for Comment
// @Description GET returns reactions for a comment.
// @Tags Reaction
// @Accept json
// @Produce json
// @Param commentID path int true "Comment ID"
// @Success 200 {array} response.Reaction
// @Failure 400 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /comments/{commentID}/reactions [get]
func (h *HTTPHandler) handleCommentReactions(c *gin.Context) {
	commentID, ok := parsePathID(c, "commentID", "comment")
	if !ok {
		return
	}
	h.handleReactionsByTarget(c, commentID, entity.ReactionTargetComment)
}

// handleMyPostReactionPut godoc
// @ID setMyPostReaction
// @Summary Set My Reaction for Post
// @Description PUT creates or updates the current user's reaction on a post.
// @Tags Reaction
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param postID path int true "Post ID"
// @Param request body reactionRequest true "Set reaction payload"
// @Success 201
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /posts/{postID}/reactions/me [put]
func (h *HTTPHandler) handleMyPostReactionPut(c *gin.Context) {
	postID, ok := parsePathID(c, "postID", "post")
	if !ok {
		return
	}
	h.handleMyReactionPut(c, postID, entity.ReactionTargetPost)
}

// handleMyPostReactionDelete godoc
// @ID deleteMyPostReaction
// @Summary Delete My Reaction for Post
// @Description DELETE removes the current user's reaction on a post.
// @Tags Reaction
// @Produce json
// @Security BearerAuth
// @Param postID path int true "Post ID"
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /posts/{postID}/reactions/me [delete]
func (h *HTTPHandler) handleMyPostReactionDelete(c *gin.Context) {
	postID, ok := parsePathID(c, "postID", "post")
	if !ok {
		return
	}
	h.handleMyReactionDelete(c, postID, entity.ReactionTargetPost)
}

// handleMyCommentReactionPut godoc
// @ID setMyCommentReaction
// @Summary Set My Reaction for Comment
// @Description PUT creates or updates the current user's reaction on a comment.
// @Tags Reaction
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param commentID path int true "Comment ID"
// @Param request body reactionRequest true "Set reaction payload"
// @Success 201
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /comments/{commentID}/reactions/me [put]
func (h *HTTPHandler) handleMyCommentReactionPut(c *gin.Context) {
	commentID, ok := parsePathID(c, "commentID", "comment")
	if !ok {
		return
	}
	h.handleMyReactionPut(c, commentID, entity.ReactionTargetComment)
}

// handleMyCommentReactionDelete godoc
// @ID deleteMyCommentReaction
// @Summary Delete My Reaction for Comment
// @Description DELETE removes the current user's reaction on a comment.
// @Tags Reaction
// @Produce json
// @Security BearerAuth
// @Param commentID path int true "Comment ID"
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /comments/{commentID}/reactions/me [delete]
func (h *HTTPHandler) handleMyCommentReactionDelete(c *gin.Context) {
	commentID, ok := parsePathID(c, "commentID", "comment")
	if !ok {
		return
	}
	h.handleMyReactionDelete(c, commentID, entity.ReactionTargetComment)
}

func (h *HTTPHandler) handleReactionsByTarget(c *gin.Context, targetID int64, targetType entity.ReactionTargetType) {
	reactions, err := h.reactionUseCase.GetReactionsByTarget(targetID, targetType)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.ReactionsFromDTO(reactions))
}

func (h *HTTPHandler) handleMyReactionPut(c *gin.Context, targetID int64, targetType entity.ReactionTargetType) {
	userID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	var req reactionRequest
	if err := decodeJSON(c, &req); err != nil {
		badRequest(c, err)
		return
	}
	reactionType, err := req.parseType()
	if err != nil {
		badRequest(c, err)
		return
	}
	created, err := h.reactionUseCase.SetReaction(userID, targetID, targetType, reactionType)
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

func (h *HTTPHandler) handleMyReactionDelete(c *gin.Context, targetID int64, targetType entity.ReactionTargetType) {
	userID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	if err := h.reactionUseCase.DeleteReaction(userID, targetID, targetType); err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func writeUseCaseError(c *gin.Context, err error) {
	publicErr := customError.Public(err)
	writeHTTPError(loggerFromContext(c), c, statusForError(err), publicErr)
}

func writeHTTPError(logger port.Logger, c *gin.Context, status int, err error) {
	publicErr := customError.Public(err)
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

func loggerFromContext(c *gin.Context) port.Logger {
	if c != nil {
		if v, ok := c.Get(httpLoggerContextKey); ok {
			if logger, ok := v.(port.Logger); ok && logger != nil {
				return logger
			}
		}
	}
	return noopLogger{}
}

func statusForError(err error) int {
	switch {
	case errors.Is(err, customError.ErrUserAlreadyExists):
		return http.StatusConflict
	case errors.Is(err, customError.ErrInvalidInput):
		return http.StatusBadRequest
	case errors.Is(err, customError.ErrUserNotFound):
		return http.StatusNotFound
	case errors.Is(err, customError.ErrBoardNotFound):
		return http.StatusNotFound
	case errors.Is(err, customError.ErrBoardNotEmpty):
		return http.StatusConflict
	case errors.Is(err, customError.ErrPostNotFound):
		return http.StatusNotFound
	case errors.Is(err, customError.ErrTagNotFound):
		return http.StatusNotFound
	case errors.Is(err, customError.ErrAttachmentNotFound):
		return http.StatusNotFound
	case errors.Is(err, customError.ErrCommentNotFound):
		return http.StatusNotFound
	case errors.Is(err, customError.ErrReactionNotFound):
		return http.StatusNotFound
	case errors.Is(err, customError.ErrInvalidCredential):
		return http.StatusUnauthorized
	case errors.Is(err, customError.ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, customError.ErrMissingAuthHeader):
		return http.StatusUnauthorized
	case errors.Is(err, customError.ErrInvalidToken):
		return http.StatusUnauthorized
	case errors.Is(err, customError.ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, customError.ErrUserSuspended):
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}

func parseLimitLastID(c *gin.Context) (int, int64, bool) {
	limitStr := c.Query("limit")
	lastIDStr := c.Query("last_id")

	limit := 10
	var lastID int64

	if limitStr != "" {
		v, err := strconv.Atoi(limitStr)
		if err != nil || v < 1 {
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

func decodeJSON(c *gin.Context, dst any) error {
	defer c.Request.Body.Close()
	dec := json.NewDecoder(c.Request.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); err != nil {
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
