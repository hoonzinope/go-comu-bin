package delivery

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/delivery/middleware"
	"github.com/hoonzinope/go-comu-bin/internal/delivery/response"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

type HTTPHandler struct {
	sessionUseCase    port.SessionUseCase
	userUseCase       port.UserUseCase
	boardUseCase      port.BoardUseCase
	postUseCase       port.PostUseCase
	commentUseCase    port.CommentUseCase
	reactionUseCase   port.ReactionUseCase
	authGinMiddleware gin.HandlerFunc
}

type HTTPDependencies struct {
	SessionUseCase  port.SessionUseCase
	UserUseCase     port.UserUseCase
	BoardUseCase    port.BoardUseCase
	PostUseCase     port.PostUseCase
	CommentUseCase  port.CommentUseCase
	ReactionUseCase port.ReactionUseCase
}

func NewHTTPHandler(deps HTTPDependencies) *HTTPHandler {
	return &HTTPHandler{
		sessionUseCase:    deps.SessionUseCase,
		userUseCase:       deps.UserUseCase,
		boardUseCase:      deps.BoardUseCase,
		postUseCase:       deps.PostUseCase,
		commentUseCase:    deps.CommentUseCase,
		reactionUseCase:   deps.ReactionUseCase,
		authGinMiddleware: middleware.AuthWithSession(deps.SessionUseCase),
	}
}

func (h *HTTPHandler) RegisterRoutes(r *gin.Engine) {
	r.HandleMethodNotAllowed = true
	r.NoMethod(func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
	})
	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, errorResponse{Error: "not found"})
	})
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	v1 := r.Group("/api/v1")
	v1.POST("/signup", h.handleUserSignUp)
	v1.POST("/auth/login", h.handleUserLogin)
	v1.POST("/auth/logout", h.authGinMiddleware, h.handleUserLogout)
	v1.DELETE("/users/me", h.authGinMiddleware, h.handleUserDeleteMe)

	v1.GET("/boards", h.handleBoardsGet)
	v1.POST("/boards", h.authGinMiddleware, h.handleBoardsPost)
	v1.PUT("/boards/:boardID", h.authGinMiddleware, h.handleBoardPut)
	v1.DELETE("/boards/:boardID", h.authGinMiddleware, h.handleBoardDelete)

	v1.GET("/boards/:boardID/posts", h.handleBoardPostsGet)
	v1.POST("/boards/:boardID/posts", h.authGinMiddleware, h.handleBoardPostsPost)

	v1.GET("/posts/:postID", h.handlePostDetailGet)
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
	if err := h.userUseCase.DeleteMe(userID, req.Password); err != nil {
		writeUseCaseError(c, err)
		return
	}
	if err := h.sessionUseCase.InvalidateUserSessions(userID); err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// handleBoardsGet godoc
// @Summary List Boards or Create Board
// @Description GET returns board list with cursor pagination, POST creates a board (admin only).
// @Tags Board
// @Accept json
// @Produce json
// @Param limit query int false "Page size" minimum(0)
// @Param last_id query int false "Cursor id, fetch items with id < last_id" minimum(0)
// @Param request body boardRequest false "Create board payload (POST only)"
// @Success 200 {object} response.BoardList
// @Success 201 {object} idResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
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
// @Summary List Boards or Create Board
// @Description GET returns board list with cursor pagination, POST creates a board (admin only).
// @Tags Board
// @Accept json
// @Produce json
// @Param request body boardRequest false "Create board payload (POST only)"
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
// @Summary Update or Delete Board
// @Description Update/delete board by id (admin only).
// @Tags Board
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param boardID path int true "Board ID"
// @Param request body boardRequest false "Update board payload (PUT only)"
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
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
// @Summary Update or Delete Board
// @Description Update/delete board by id (admin only).
// @Tags Board
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param boardID path int true "Board ID"
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
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
// @Summary List Posts by Board or Create Post
// @Description GET returns posts in board with cursor pagination, POST creates post in board.
// @Tags Post
// @Accept json
// @Produce json
// @Param boardID path int true "Board ID"
// @Param limit query int false "Page size" minimum(0)
// @Param last_id query int false "Cursor id, fetch items with id < last_id" minimum(0)
// @Param request body postRequest false "Create post payload (POST only)"
// @Success 200 {object} response.PostList
// @Success 201 {object} idResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
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
// @Summary List Posts by Board or Create Post
// @Description GET returns posts in board with cursor pagination, POST creates post in board.
// @Tags Post
// @Accept json
// @Produce json
// @Param boardID path int true "Board ID"
// @Param request body postRequest false "Create post payload (POST only)"
// @Success 201 {object} idResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
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
	postID, err := h.postUseCase.CreatePost(req.Title, req.Content, authorID, boardID)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.JSON(http.StatusCreated, idResponse{ID: postID})
}

// handlePostDetailGet godoc
// @Summary Get, Update or Delete Post
// @Description Retrieve post detail or mutate post by id.
// @Tags Post
// @Accept json
// @Produce json
// @Param postID path int true "Post ID"
// @Param request body postRequest false "Update post payload (PUT only)"
// @Success 200 {object} response.PostDetail
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
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

// handlePostDetailPut godoc
// @Summary Get, Update or Delete Post
// @Description Retrieve post detail or mutate post by id.
// @Tags Post
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param postID path int true "Post ID"
// @Param request body postRequest false "Update post payload (PUT only)"
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
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
	if err := h.postUseCase.UpdatePost(postID, authorID, req.Title, req.Content); err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// handlePostDetailDelete godoc
// @Summary Get, Update or Delete Post
// @Description Retrieve post detail or mutate post by id.
// @Tags Post
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param postID path int true "Post ID"
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
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
// @Summary List Comments by Post or Create Comment
// @Description GET returns comments in post with cursor pagination, POST creates comment.
// @Tags Comment
// @Accept json
// @Produce json
// @Param postID path int true "Post ID"
// @Param limit query int false "Page size" minimum(0)
// @Param last_id query int false "Cursor id, fetch items with id < last_id" minimum(0)
// @Param request body commentRequest false "Create comment payload (POST only)"
// @Success 200 {object} response.CommentList
// @Success 201 {object} idResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
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
// @Summary List Comments by Post or Create Comment
// @Description GET returns comments in post with cursor pagination, POST creates comment.
// @Tags Comment
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param postID path int true "Post ID"
// @Param request body commentRequest false "Create comment payload (POST only)"
// @Success 201 {object} idResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
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
	id, err := h.commentUseCase.CreateComment(req.Content, authorID, postID)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.JSON(http.StatusCreated, idResponse{ID: id})
}

// handleCommentPut godoc
// @Summary Update or Delete Comment
// @Description Update/delete comment by id.
// @Tags Comment
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param commentID path int true "Comment ID"
// @Param request body commentRequest false "Update comment payload (PUT only)"
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
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
// @Summary Update or Delete Comment
// @Description Update/delete comment by id.
// @Tags Comment
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param commentID path int true "Comment ID"
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
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
	status := statusForError(err)
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
		slog.Error("request failed", logAttrs...)
	} else {
		slog.Warn("request failed", logAttrs...)
	}
	c.JSON(status, errorResponse{Error: publicErr.Error()})
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
	case errors.Is(err, customError.ErrPostNotFound):
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
		if err != nil || v < 0 {
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
	return nil
}

func badRequest(c *gin.Context, err error) {
	c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
}
