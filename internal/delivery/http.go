package delivery

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/hoonzinope/go-comu-bin/internal/application"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/delivery/middleware"
	"github.com/hoonzinope/go-comu-bin/internal/delivery/response"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

type HTTPHandler struct {
	authUseCase       application.AuthUseCase
	cache             application.Cache
	userUseCase       application.UserUseCase
	boardUseCase      application.BoardUseCase
	postUseCase       application.PostUseCase
	commentUseCase    application.CommentUseCase
	reactionUseCase   application.ReactionUseCase
	authGinMiddleware gin.HandlerFunc
}

type userCredentialRequest struct {
	Username string `json:"username" example:"alice"`
	Password string `json:"password" example:"pw"`
}

type passwordOnlyRequest struct {
	Password string `json:"password" example:"pw"`
}

type signUpResponse struct {
	Result string `json:"result" example:"ok"`
}

type loginResponse struct {
	Login string `json:"login" example:"ok"`
}

type logoutResponse struct {
	Logout string `json:"logout" example:"ok"`
}

type errorResponse struct {
	Error string `json:"error" example:"invalid credential"`
}

type idResponse struct {
	ID int64 `json:"id" example:"1"`
}

type boardRequest struct {
	Name        string `json:"name" example:"free"`
	Description string `json:"description" example:"free board"`
}

type postRequest struct {
	Title   string `json:"title" example:"hello"`
	Content string `json:"content" example:"first post"`
}

type commentRequest struct {
	Content string `json:"content" example:"nice post"`
}

type reactionRequest struct {
	ReactionType string `json:"reaction_type" example:"like"`
}

func NewHTTPHandler(useCase application.UseCase, authUseCase application.AuthUseCase, cache application.Cache) *HTTPHandler {
	return &HTTPHandler{
		authUseCase:       authUseCase,
		cache:             cache,
		userUseCase:       useCase.UserUseCase,
		boardUseCase:      useCase.BoardUseCase,
		postUseCase:       useCase.PostUseCase,
		commentUseCase:    useCase.CommentUseCase,
		reactionUseCase:   useCase.ReactionUseCase,
		authGinMiddleware: middleware.AuthWithCache(authUseCase, cache),
	}
}

func (h *HTTPHandler) RegisterRoutes(r *gin.Engine) {
	r.HandleMethodNotAllowed = true
	r.NoMethod(func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not allowed"})
	})
	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	})
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	v1 := r.Group("/api/v1")
	v1.POST("/signup", h.handleUserSignUp)
	v1.POST("/auth/login", h.handleUserLogin)
	v1.POST("/auth/logout", h.authGinMiddleware, h.handleUserLogout)
	v1.DELETE("/users/me", h.authGinMiddleware, h.handleUserDeleteMe)

	v1.GET("/boards", h.handleBoards)
	v1.POST("/boards", h.authGinMiddleware, h.handleBoards)
	v1.PUT("/boards/:boardID", h.authGinMiddleware, h.handleBoardWithID)
	v1.DELETE("/boards/:boardID", h.authGinMiddleware, h.handleBoardWithID)

	v1.GET("/boards/:boardID/posts", h.handleBoardPosts)
	v1.POST("/boards/:boardID/posts", h.authGinMiddleware, h.handleBoardPosts)

	v1.GET("/posts/:postID", h.handlePostDetail)
	v1.PUT("/posts/:postID", h.authGinMiddleware, h.handlePostDetail)
	v1.DELETE("/posts/:postID", h.authGinMiddleware, h.handlePostDetail)

	v1.GET("/posts/:postID/comments", h.handlePostComments)
	v1.POST("/posts/:postID/comments", h.authGinMiddleware, h.handlePostComments)
	v1.GET("/posts/:postID/reactions", h.handlePostReactions)
	v1.POST("/posts/:postID/reactions", h.authGinMiddleware, h.handlePostReactions)

	v1.PUT("/comments/:commentID", h.authGinMiddleware, h.handleComments)
	v1.DELETE("/comments/:commentID", h.authGinMiddleware, h.handleComments)
	v1.GET("/comments/:commentID/reactions", h.handleCommentReactions)
	v1.POST("/comments/:commentID/reactions", h.authGinMiddleware, h.handleCommentReactions)
	v1.DELETE("/reactions/:reactionID", h.authGinMiddleware, h.handleReactionWithID)
}

func NewHTTPServer(addr string, authUseCase application.AuthUseCase, cache application.Cache, useCase application.UseCase) *http.Server {
	r := gin.New()
	r.Use(gin.Recovery())
	handler := NewHTTPHandler(useCase, authUseCase, cache)
	handler.RegisterRoutes(r)
	return &http.Server{Addr: addr, Handler: r}
}

func (h *HTTPHandler) requireAuthUserID(c *gin.Context) (int64, bool) {
	userID, ok := middleware.UserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": customError.ErrUnauthorized.Error()})
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
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(c, &req); err != nil {
		badRequest(c, err)
		return
	}
	if req.Username == "" || req.Password == "" {
		badRequest(c, errors.New("username and password are required"))
		return
	}
	if _, err := h.userUseCase.SignUp(req.Username, req.Password); err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"result": "ok"})
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
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /auth/login [post]
func (h *HTTPHandler) handleUserLogin(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(c, &req); err != nil {
		badRequest(c, err)
		return
	}
	userID, err := h.userUseCase.Login(req.Username, req.Password)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	token, err := h.authUseCase.IdToToken(userID)
	if err != nil {
		writeUseCaseError(c, err)
		return
	}
	h.cache.Set(token, userID)
	c.Header("Authorization", "Bearer "+token)
	c.JSON(http.StatusOK, gin.H{"login": "ok"})
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
		c.JSON(http.StatusUnauthorized, gin.H{"error": customError.ErrUnauthorized.Error()})
		return
	}
	h.cache.Delete(token)
	c.JSON(http.StatusOK, gin.H{"logout": "ok"})
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
	var req struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(c, &req); err != nil {
		badRequest(c, err)
		return
	}
	if req.Password == "" {
		badRequest(c, errors.New("password is required"))
		return
	}
	if err := h.userUseCase.DeleteMe(userID, req.Password); err != nil {
		writeUseCaseError(c, err)
		return
	}
	if token, exists := middleware.Token(c); exists {
		h.cache.Delete(token)
	}
	c.Status(http.StatusNoContent)
}

// handleBoards godoc
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
// @Router /boards [post]
func (h *HTTPHandler) handleBoards(c *gin.Context) {
	switch c.Request.Method {
	case http.MethodGet:
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
	case http.MethodPost:
		userID, ok := h.requireAuthUserID(c)
		if !ok {
			return
		}
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := decodeJSON(c, &req); err != nil {
			badRequest(c, err)
			return
		}
		if userID == 0 || req.Name == "" {
			badRequest(c, errors.New("user_id and name are required"))
			return
		}
		id, err := h.boardUseCase.CreateBoard(userID, req.Name, req.Description)
		if err != nil {
			writeUseCaseError(c, err)
			return
		}
		c.JSON(http.StatusCreated, map[string]int64{"id": id})
	}
}

// handleBoardWithID godoc
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
// @Router /boards/{boardID} [delete]
func (h *HTTPHandler) handleBoardWithID(c *gin.Context) {
	boardID, err := parseInt64(c.Param("boardID"))
	if err != nil {
		badRequest(c, errors.New("invalid board id"))
		return
	}

	switch c.Request.Method {
	case http.MethodPut:
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := decodeJSON(c, &req); err != nil {
			badRequest(c, err)
			return
		}
		userID, ok := h.requireAuthUserID(c)
		if !ok {
			return
		}
		if userID == 0 || req.Name == "" {
			badRequest(c, errors.New("user_id and name are required"))
			return
		}
		if err := h.boardUseCase.UpdateBoard(boardID, userID, req.Name, req.Description); err != nil {
			writeUseCaseError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	case http.MethodDelete:
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
}

// handleBoardPosts godoc
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
// @Router /boards/{boardID}/posts [post]
func (h *HTTPHandler) handleBoardPosts(c *gin.Context) {
	boardID, err := parseInt64(c.Param("boardID"))
	if err != nil {
		badRequest(c, errors.New("invalid board id"))
		return
	}

	switch c.Request.Method {
	case http.MethodGet:
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
	case http.MethodPost:
		authorID, ok := h.requireAuthUserID(c)
		if !ok {
			return
		}
		var req struct {
			Title   string `json:"title"`
			Content string `json:"content"`
		}
		if err := decodeJSON(c, &req); err != nil {
			badRequest(c, err)
			return
		}
		if authorID == 0 || req.Title == "" || req.Content == "" {
			badRequest(c, errors.New("author_id, title and content are required"))
			return
		}
		postID, err := h.postUseCase.CreatePost(req.Title, req.Content, authorID, boardID)
		if err != nil {
			writeUseCaseError(c, err)
			return
		}
		c.JSON(http.StatusCreated, map[string]int64{"id": postID})
	}
}

// handlePostDetail godoc
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
// @Router /posts/{postID} [put]
// @Router /posts/{postID} [delete]
func (h *HTTPHandler) handlePostDetail(c *gin.Context) {
	postID, err := parseInt64(c.Param("postID"))
	if err != nil {
		badRequest(c, errors.New("invalid post id"))
		return
	}

	switch c.Request.Method {
	case http.MethodGet:
		post, err := h.postUseCase.GetPostDetail(postID)
		if err != nil {
			writeUseCaseError(c, err)
			return
		}
		c.JSON(http.StatusOK, response.PostDetailFromDTO(post))
	case http.MethodPut:
		authorID, ok := h.requireAuthUserID(c)
		if !ok {
			return
		}
		var req struct {
			Title   string `json:"title"`
			Content string `json:"content"`
		}
		if err := decodeJSON(c, &req); err != nil {
			badRequest(c, err)
			return
		}
		if authorID == 0 || req.Title == "" || req.Content == "" {
			badRequest(c, errors.New("author_id, title and content are required"))
			return
		}
		if err := h.postUseCase.UpdatePost(postID, authorID, req.Title, req.Content); err != nil {
			writeUseCaseError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	case http.MethodDelete:
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
}

// handlePostComments godoc
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
// @Router /posts/{postID}/comments [post]
func (h *HTTPHandler) handlePostComments(c *gin.Context) {
	postID, err := parseInt64(c.Param("postID"))
	if err != nil {
		badRequest(c, errors.New("invalid post id"))
		return
	}

	switch c.Request.Method {
	case http.MethodGet:
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
	case http.MethodPost:
		authorID, ok := h.requireAuthUserID(c)
		if !ok {
			return
		}
		var req struct {
			Content string `json:"content"`
		}
		if err := decodeJSON(c, &req); err != nil {
			badRequest(c, err)
			return
		}
		if authorID == 0 || req.Content == "" {
			badRequest(c, errors.New("author_id and content are required"))
			return
		}
		id, err := h.commentUseCase.CreateComment(req.Content, authorID, postID)
		if err != nil {
			writeUseCaseError(c, err)
			return
		}
		c.JSON(http.StatusCreated, map[string]int64{"id": id})
	}
}

// handleComments godoc
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
// @Router /comments/{commentID} [delete]
func (h *HTTPHandler) handleComments(c *gin.Context) {
	commentID, err := parseInt64(c.Param("commentID"))
	if err != nil {
		badRequest(c, errors.New("invalid comment id"))
		return
	}

	switch c.Request.Method {
	case http.MethodPut:
		authorID, ok := h.requireAuthUserID(c)
		if !ok {
			return
		}
		var req struct {
			Content string `json:"content"`
		}
		if err := decodeJSON(c, &req); err != nil {
			badRequest(c, err)
			return
		}
		if authorID == 0 || req.Content == "" {
			badRequest(c, errors.New("author_id and content are required"))
			return
		}
		if err := h.commentUseCase.UpdateComment(commentID, authorID, req.Content); err != nil {
			writeUseCaseError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	case http.MethodDelete:
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
}

// handlePostReactions godoc
// @Summary List or Create Reactions for Post
// @Description GET returns reactions for a post, POST creates reaction on a post.
// @Tags Reaction
// @Accept json
// @Produce json
// @Param postID path int true "Post ID"
// @Param request body reactionRequest false "Create reaction payload (POST only)"
// @Success 200 {array} response.Reaction
// @Success 201
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /posts/{postID}/reactions [get]
// @Router /posts/{postID}/reactions [post]
func (h *HTTPHandler) handlePostReactions(c *gin.Context) {
	postID, err := parseInt64(c.Param("postID"))
	if err != nil {
		badRequest(c, errors.New("invalid post id"))
		return
	}
	h.handleReactionsByTarget(c, postID, "post")
}

// handleCommentReactions godoc
// @Summary List or Create Reactions for Comment
// @Description GET returns reactions for a comment, POST creates reaction on a comment.
// @Tags Reaction
// @Accept json
// @Produce json
// @Param commentID path int true "Comment ID"
// @Param request body reactionRequest false "Create reaction payload (POST only)"
// @Success 200 {array} response.Reaction
// @Success 201
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /comments/{commentID}/reactions [get]
// @Router /comments/{commentID}/reactions [post]
func (h *HTTPHandler) handleCommentReactions(c *gin.Context) {
	commentID, err := parseInt64(c.Param("commentID"))
	if err != nil {
		badRequest(c, errors.New("invalid comment id"))
		return
	}
	h.handleReactionsByTarget(c, commentID, "comment")
}

func (h *HTTPHandler) handleReactionsByTarget(c *gin.Context, targetID int64, targetType string) {
	switch c.Request.Method {
	case http.MethodGet:
		reactions, err := h.reactionUseCase.GetReactionsByTarget(targetID, targetType)
		if err != nil {
			writeUseCaseError(c, err)
			return
		}
		c.JSON(http.StatusOK, response.ReactionsFromEntities(reactions))
	case http.MethodPost:
		userID, ok := h.requireAuthUserID(c)
		if !ok {
			return
		}
		var req struct {
			ReactionType string `json:"reaction_type"`
		}
		if err := decodeJSON(c, &req); err != nil {
			badRequest(c, err)
			return
		}
		if userID == 0 || req.ReactionType == "" {
			badRequest(c, errors.New("user_id and reaction_type are required"))
			return
		}
		if err := h.reactionUseCase.AddReaction(userID, targetID, targetType, req.ReactionType); err != nil {
			writeUseCaseError(c, err)
			return
		}
		c.Status(http.StatusCreated)
	}
}

// handleReactionWithID godoc
// @Summary Delete Reaction
// @Description Delete reaction by reaction id.
// @Tags Reaction
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param reactionID path int true "Reaction ID"
// @Success 204
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /reactions/{reactionID} [delete]
func (h *HTTPHandler) handleReactionWithID(c *gin.Context) {
	reactionID, err := parseInt64(c.Param("reactionID"))
	if err != nil {
		badRequest(c, errors.New("invalid reaction id"))
		return
	}
	userID, ok := h.requireAuthUserID(c)
	if !ok {
		return
	}
	if err := h.reactionUseCase.RemoveReaction(userID, reactionID); err != nil {
		writeUseCaseError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func writeUseCaseError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, customError.ErrUserAlreadyExists):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, customError.ErrUserNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, customError.ErrBoardNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, customError.ErrPostNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, customError.ErrCommentNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, customError.ErrReactionNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, customError.ErrInvalidCredential):
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
	case errors.Is(err, customError.ErrUnauthorized):
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
	case errors.Is(err, customError.ErrMissingAuthHeader):
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
	case errors.Is(err, customError.ErrInvalidToken):
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
	case errors.Is(err, customError.ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
}
