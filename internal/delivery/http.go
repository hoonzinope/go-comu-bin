package delivery

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/hoonzinope/go-comu-bin/internal/application"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/delivery/middleware"
	"github.com/hoonzinope/go-comu-bin/internal/infrastructure/auth"
)

type HTTPHandler struct {
	authMiddleware  *middleware.AuthMiddleware
	userUseCase     application.UserUseCase
	boardUseCase    application.BoardUseCase
	postUseCase     application.PostUseCase
	commentUseCase  application.CommentUseCase
	reactionUseCase application.ReactionUseCase
}

func NewHTTPHandler(useCase application.UseCase, authMiddleware *middleware.AuthMiddleware) *HTTPHandler {
	return &HTTPHandler{
		authMiddleware:  authMiddleware,
		userUseCase:     useCase.UserUseCase,
		boardUseCase:    useCase.BoardUseCase,
		postUseCase:     useCase.PostUseCase,
		commentUseCase:  useCase.CommentUseCase,
		reactionUseCase: useCase.ReactionUseCase,
	}
}

func (h *HTTPHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/users/signup", h.handleUserSignUp)
	mux.HandleFunc("/users/login", h.handleUserLogin)
	mux.Handle("/users/logout", h.authMiddleware.AuthByMethods(http.HandlerFunc(h.handleUserLogout), http.MethodPost))
	mux.HandleFunc("/users/quit", h.handleUserQuit)

	mux.Handle("/boards", h.authMiddleware.AuthByMethods(http.HandlerFunc(h.handleBoards), http.MethodPost))
	mux.Handle("/boards/", h.authMiddleware.AuthByMethods(http.HandlerFunc(h.handleBoardWithID), http.MethodPut, http.MethodDelete, http.MethodPost))
	mux.Handle("/posts/", h.authMiddleware.AuthByMethods(http.HandlerFunc(h.handlePosts), http.MethodPut, http.MethodDelete, http.MethodPost))
	mux.Handle("/comments/", h.authMiddleware.AuthByMethods(http.HandlerFunc(h.handleComments), http.MethodPut, http.MethodDelete))
	mux.Handle("/reactions", h.authMiddleware.AuthByMethods(http.HandlerFunc(h.handleReactions), http.MethodPost))
	mux.Handle("/reactions/", h.authMiddleware.AuthByMethods(http.HandlerFunc(h.handleReactionWithID), http.MethodDelete))
}

func NewHTTPServer(addr string, jwtSecret string, useCase application.UseCase) *http.Server {
	mux := http.NewServeMux()
	authMiddleware := middleware.NewAuthMiddleware(auth.NewJwtTokenProvider(jwtSecret))
	handler := NewHTTPHandler(useCase, authMiddleware)
	handler.RegisterRoutes(mux)
	return &http.Server{Addr: addr, Handler: mux}
}

func (h *HTTPHandler) requireAuthUserID(w http.ResponseWriter, r *http.Request) (int64, error) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, customError.ErrUnauthorized.Error(), http.StatusUnauthorized)
		return 0, customError.ErrUnauthorized
	}
	return userID, nil
}

func (h *HTTPHandler) handleUserSignUp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	if req.Username == "" || req.Password == "" {
		badRequest(w, errors.New("username and password are required"))
		return
	}
	_, err := h.userUseCase.SignUp(req.Username, req.Password)
	if err != nil {
		writeUseCaseError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"result": "ok"})
}

func (h *HTTPHandler) handleUserLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	userID, err := h.userUseCase.Login(req.Username, req.Password)
	if err != nil {
		writeUseCaseError(w, err)
		return
	}
	token, err := h.authMiddleware.AuthUseCase.IdToToken(userID)
	if err != nil {
		writeUseCaseError(w, err)
		return
	}
	// header에 토큰을 담아서 반환
	w.Header().Set("Authorization", "Bearer "+token)
	writeJSON(w, http.StatusOK, map[string]string{"login": "ok"})
}

func (h *HTTPHandler) handleUserLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	_, err := h.requireAuthUserID(w, r)
	if err != nil {
		return
	}
	// 여기서는 간단히 성공 응답만 반환
	writeJSON(w, http.StatusOK, map[string]string{"logout": "ok"})
}

func (h *HTTPHandler) handleUserQuit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		methodNotAllowed(w)
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w, err)
		return
	}
	if req.Username == "" || req.Password == "" {
		badRequest(w, errors.New("username and password are required"))
		return
	}
	if err := h.userUseCase.Quit(req.Username, req.Password); err != nil {
		writeUseCaseError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *HTTPHandler) handleBoards(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		limit, offset, ok := parseLimitOffset(w, r)
		if !ok {
			return
		}
		boards, err := h.boardUseCase.GetBoards(limit, offset)
		if err != nil {
			writeUseCaseError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, boards)
	case http.MethodPost:
		userID, err := h.requireAuthUserID(w, r)
		if err != nil {
			return
		}
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := decodeJSON(r, &req); err != nil {
			badRequest(w, err)
			return
		}
		if userID == 0 || req.Name == "" {
			badRequest(w, errors.New("user_id and name are required"))
			return
		}
		id, err := h.boardUseCase.CreateBoard(userID, req.Name, req.Description)
		if err != nil {
			writeUseCaseError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
	default:
		methodNotAllowed(w)
	}
}

func (h *HTTPHandler) handleBoardWithID(w http.ResponseWriter, r *http.Request) {
	segments := splitPath(r.URL.Path)
	if len(segments) < 2 || segments[0] != "boards" {
		notFound(w)
		return
	}
	boardID, err := parseInt64(segments[1])
	if err != nil {
		badRequest(w, errors.New("invalid board id"))
		return
	}
	if len(segments) == 3 && segments[2] == "posts" {
		h.handleBoardPosts(w, r, boardID)
		return
	}
	if len(segments) != 2 {
		notFound(w)
		return
	}

	switch r.Method {
	case http.MethodPut:
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := decodeJSON(r, &req); err != nil {
			badRequest(w, err)
			return
		}
		userID, err := h.requireAuthUserID(w, r)
		if err != nil {
			return
		}
		if userID == 0 || req.Name == "" {
			badRequest(w, errors.New("user_id and name are required"))
			return
		}
		if err := h.boardUseCase.UpdateBoard(boardID, userID, req.Name, req.Description); err != nil {
			writeUseCaseError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		userID, err := h.requireAuthUserID(w, r)
		if err != nil {
			return
		}
		if err := h.boardUseCase.DeleteBoard(boardID, userID); err != nil {
			writeUseCaseError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		methodNotAllowed(w)
	}
}

func (h *HTTPHandler) handleBoardPosts(w http.ResponseWriter, r *http.Request, boardID int64) {
	switch r.Method {
	case http.MethodGet:
		limit, offset, ok := parseLimitOffset(w, r)
		if !ok {
			return
		}
		posts, err := h.postUseCase.GetPostsList(boardID, limit, offset)
		if err != nil {
			writeUseCaseError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, posts)
	case http.MethodPost:
		authorID, err := h.requireAuthUserID(w, r)
		if err != nil {
			return
		}
		var req struct {
			Title   string `json:"title"`
			Content string `json:"content"`
		}
		if err := decodeJSON(r, &req); err != nil {
			badRequest(w, err)
			return
		}
		if authorID == 0 || req.Title == "" || req.Content == "" {
			badRequest(w, errors.New("author_id, title and content are required"))
			return
		}
		postID, err := h.postUseCase.CreatePost(req.Title, req.Content, authorID, boardID)
		if err != nil {
			writeUseCaseError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]int64{"id": postID})
	default:
		methodNotAllowed(w)
	}
}

func (h *HTTPHandler) handlePosts(w http.ResponseWriter, r *http.Request) {
	segments := splitPath(r.URL.Path)
	if len(segments) < 2 || segments[0] != "posts" {
		notFound(w)
		return
	}

	postID, err := parseInt64(segments[1])
	if err != nil {
		badRequest(w, errors.New("invalid post id"))
		return
	}

	if len(segments) == 2 {
		h.handlePostDetail(w, r, postID)
		return
	}

	if len(segments) == 3 && segments[2] == "comments" {
		h.handlePostComments(w, r, postID)
		return
	}

	notFound(w)
}

func (h *HTTPHandler) handlePostDetail(w http.ResponseWriter, r *http.Request, postID int64) {
	switch r.Method {
	case http.MethodGet:
		post, err := h.postUseCase.GetPostDetail(postID)
		if err != nil {
			writeUseCaseError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, post)
	case http.MethodPut:
		authorID, err := h.requireAuthUserID(w, r)
		if err != nil {
			return
		}
		var req struct {
			Title   string `json:"title"`
			Content string `json:"content"`
		}
		if err := decodeJSON(r, &req); err != nil {
			badRequest(w, err)
			return
		}
		if authorID == 0 || req.Title == "" || req.Content == "" {
			badRequest(w, errors.New("author_id, title and content are required"))
			return
		}
		if err := h.postUseCase.UpdatePost(postID, authorID, req.Title, req.Content); err != nil {
			writeUseCaseError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		authorID, err := h.requireAuthUserID(w, r)
		if err != nil {
			return
		}
		if err := h.postUseCase.DeletePost(postID, authorID); err != nil {
			writeUseCaseError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		methodNotAllowed(w)
	}
}

func (h *HTTPHandler) handlePostComments(w http.ResponseWriter, r *http.Request, postID int64) {
	switch r.Method {
	case http.MethodGet:
		limit, offset, ok := parseLimitOffset(w, r)
		if !ok {
			return
		}
		comments, err := h.commentUseCase.GetCommentsByPost(postID, limit, offset)
		if err != nil {
			writeUseCaseError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, comments)
	case http.MethodPost:
		authorID, err := h.requireAuthUserID(w, r)
		if err != nil {
			return
		}
		var req struct {
			Content string `json:"content"`
		}
		if err := decodeJSON(r, &req); err != nil {
			badRequest(w, err)
			return
		}
		if authorID == 0 || req.Content == "" {
			badRequest(w, errors.New("author_id and content are required"))
			return
		}
		id, err := h.commentUseCase.CreateComment(req.Content, authorID, postID)
		if err != nil {
			writeUseCaseError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
	default:
		methodNotAllowed(w)
	}
}

func (h *HTTPHandler) handleComments(w http.ResponseWriter, r *http.Request) {
	segments := splitPath(r.URL.Path)
	if len(segments) != 2 || segments[0] != "comments" {
		notFound(w)
		return
	}
	commentID, err := parseInt64(segments[1])
	if err != nil {
		badRequest(w, errors.New("invalid comment id"))
		return
	}

	switch r.Method {
	case http.MethodPut:
		authorID, err := h.requireAuthUserID(w, r)
		if err != nil {
			return
		}
		var req struct {
			Content string `json:"content"`
		}
		if err := decodeJSON(r, &req); err != nil {
			badRequest(w, err)
			return
		}
		if authorID == 0 || req.Content == "" {
			badRequest(w, errors.New("author_id and content are required"))
			return
		}
		if err := h.commentUseCase.UpdateComment(commentID, authorID, req.Content); err != nil {
			writeUseCaseError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		authorID, err := h.requireAuthUserID(w, r)
		if err != nil {
			return
		}
		if err := h.commentUseCase.DeleteComment(commentID, authorID); err != nil {
			writeUseCaseError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		methodNotAllowed(w)
	}
}

func (h *HTTPHandler) handleReactions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		targetID, err := parseInt64(r.URL.Query().Get("target_id"))
		if err != nil {
			badRequest(w, errors.New("invalid target_id"))
			return
		}
		targetType := r.URL.Query().Get("target_type")
		if targetType == "" {
			badRequest(w, errors.New("target_type is required"))
			return
		}
		reactions, err := h.reactionUseCase.GetReactionsByTarget(targetID, targetType)
		if err != nil {
			writeUseCaseError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, reactions)
	case http.MethodPost:
		userID, err := h.requireAuthUserID(w, r)
		if err != nil {
			return
		}
		var req struct {
			TargetID     int64  `json:"target_id"`
			TargetType   string `json:"target_type"`
			ReactionType string `json:"reaction_type"`
		}
		if err := decodeJSON(r, &req); err != nil {
			badRequest(w, err)
			return
		}
		if userID == 0 || req.TargetID == 0 || req.TargetType == "" || req.ReactionType == "" {
			badRequest(w, errors.New("user_id, target_id, target_type and reaction_type are required"))
			return
		}
		if err := h.reactionUseCase.AddReaction(userID, req.TargetID, req.TargetType, req.ReactionType); err != nil {
			writeUseCaseError(w, err)
			return
		}
		w.WriteHeader(http.StatusCreated)
	default:
		methodNotAllowed(w)
	}
}

func (h *HTTPHandler) handleReactionWithID(w http.ResponseWriter, r *http.Request) {
	segments := splitPath(r.URL.Path)
	if len(segments) != 2 || segments[0] != "reactions" {
		notFound(w)
		return
	}
	reactionID, err := parseInt64(segments[1])
	if err != nil {
		badRequest(w, errors.New("invalid reaction id"))
		return
	}
	if r.Method != http.MethodDelete {
		methodNotAllowed(w)
		return
	}

	userID, err := h.requireAuthUserID(w, r)
	if err != nil {
		return
	}
	if err := h.reactionUseCase.RemoveReaction(userID, reactionID); err != nil {
		writeUseCaseError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeUseCaseError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, customError.ErrUserAlreadyExists):
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
	case errors.Is(err, customError.ErrUserNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	case errors.Is(err, customError.ErrInvalidCredential):
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
	case errors.Is(err, customError.ErrUnauthorized):
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
	case errors.Is(err, customError.ErrMissingAuthHeader):
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
	case errors.Is(err, customError.ErrInvalidToken):
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
	case errors.Is(err, customError.ErrForbidden):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
}

func parseLimitOffset(w http.ResponseWriter, r *http.Request) (int, int, bool) {
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 10
	offset := 0

	if limitStr != "" {
		v, err := strconv.Atoi(limitStr)
		if err != nil || v < 0 {
			badRequest(w, errors.New("invalid limit"))
			return 0, 0, false
		}
		limit = v
	}

	if offsetStr != "" {
		v, err := strconv.Atoi(offsetStr)
		if err != nil || v < 0 {
			badRequest(w, errors.New("invalid offset"))
			return 0, 0, false
		}
		offset = v
	}

	return limit, offset, true
}

func parseInt64(raw string) (int64, error) {
	if raw == "" {
		return 0, errors.New("value is required")
	}
	return strconv.ParseInt(raw, 10, 64)
}

func splitPath(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return []string{}
	}
	return strings.Split(trimmed, "/")
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func badRequest(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
}

func methodNotAllowed(w http.ResponseWriter) {
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}

func notFound(w http.ResponseWriter) {
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
}
