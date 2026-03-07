package delivery

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/application/service"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/infrastructure/auth"
	cacheInMemory "github.com/hoonzinope/go-comu-bin/internal/infrastructure/cache/inmemory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const apiV1Prefix = "/api/v1"

type fakeUserUseCase struct {
	signUp           func(username, password string) (string, error)
	deleteMe         func(userID int64, password string) error
	verifyCredential func(username, password string) (int64, error)
}

func (f *fakeUserUseCase) SignUp(username, password string) (string, error) {
	if f.signUp != nil {
		return f.signUp(username, password)
	}
	return "ok", nil
}

func (f *fakeUserUseCase) DeleteMe(userID int64, password string) error {
	if f.deleteMe != nil {
		return f.deleteMe(userID, password)
	}
	return nil
}

func (f *fakeUserUseCase) VerifyCredentials(username, password string) (int64, error) {
	if f.verifyCredential != nil {
		return f.verifyCredential(username, password)
	}
	return 1, nil
}

type fakeBoardUseCase struct {
	getBoards   func(limit int, lastID int64) (*model.BoardList, error)
	createBoard func(userID int64, name, description string) (int64, error)
	updateBoard func(id, userID int64, name, description string) error
	deleteBoard func(id, userID int64) error
}

func (f *fakeBoardUseCase) GetBoards(limit int, lastID int64) (*model.BoardList, error) {
	if f.getBoards != nil {
		return f.getBoards(limit, lastID)
	}
	return &model.BoardList{}, nil
}

func (f *fakeBoardUseCase) CreateBoard(userID int64, name, description string) (int64, error) {
	if f.createBoard != nil {
		return f.createBoard(userID, name, description)
	}
	return 1, nil
}

func (f *fakeBoardUseCase) UpdateBoard(id, userID int64, name, description string) error {
	if f.updateBoard != nil {
		return f.updateBoard(id, userID, name, description)
	}
	return nil
}

func (f *fakeBoardUseCase) DeleteBoard(id, userID int64) error {
	if f.deleteBoard != nil {
		return f.deleteBoard(id, userID)
	}
	return nil
}

type fakePostUseCase struct {
	createPost    func(title, content string, authorID, boardID int64) (int64, error)
	getPostsList  func(boardID int64, limit int, lastID int64) (*model.PostList, error)
	getPostDetail func(postID int64) (*model.PostDetail, error)
	updatePost    func(id, authorID int64, title, content string) error
	deletePost    func(id, authorID int64) error
}

func (f *fakePostUseCase) CreatePost(title, content string, authorID, boardID int64) (int64, error) {
	if f.createPost != nil {
		return f.createPost(title, content, authorID, boardID)
	}
	return 1, nil
}

func (f *fakePostUseCase) GetPostsList(boardID int64, limit int, lastID int64) (*model.PostList, error) {
	if f.getPostsList != nil {
		return f.getPostsList(boardID, limit, lastID)
	}
	return &model.PostList{}, nil
}

func (f *fakePostUseCase) GetPostDetail(postID int64) (*model.PostDetail, error) {
	if f.getPostDetail != nil {
		return f.getPostDetail(postID)
	}
	return &model.PostDetail{}, nil
}

func (f *fakePostUseCase) UpdatePost(id, authorID int64, title, content string) error {
	if f.updatePost != nil {
		return f.updatePost(id, authorID, title, content)
	}
	return nil
}

func (f *fakePostUseCase) DeletePost(id, authorID int64) error {
	if f.deletePost != nil {
		return f.deletePost(id, authorID)
	}
	return nil
}

type fakeCommentUseCase struct {
	createComment     func(content string, authorID, postID int64) (int64, error)
	getCommentsByPost func(postID int64, limit int, lastID int64) (*model.CommentList, error)
	updateComment     func(id, authorID int64, content string) error
	deleteComment     func(id, authorID int64) error
}

func (f *fakeCommentUseCase) CreateComment(content string, authorID, postID int64) (int64, error) {
	if f.createComment != nil {
		return f.createComment(content, authorID, postID)
	}
	return 1, nil
}

func (f *fakeCommentUseCase) GetCommentsByPost(postID int64, limit int, lastID int64) (*model.CommentList, error) {
	if f.getCommentsByPost != nil {
		return f.getCommentsByPost(postID, limit, lastID)
	}
	return &model.CommentList{}, nil
}

func (f *fakeCommentUseCase) UpdateComment(id, authorID int64, content string) error {
	if f.updateComment != nil {
		return f.updateComment(id, authorID, content)
	}
	return nil
}

func (f *fakeCommentUseCase) DeleteComment(id, authorID int64) error {
	if f.deleteComment != nil {
		return f.deleteComment(id, authorID)
	}
	return nil
}

type fakeReactionUseCase struct {
	addReaction          func(userID, targetID int64, targetType, reactionType string) error
	removeReaction       func(userID, id int64) error
	getReactionsByTarget func(targetID int64, targetType string) ([]model.Reaction, error)
}

var testCache port.Cache

type authUserPort interface {
	port.UserUseCase
	port.CredentialVerifier
}

func (f *fakeReactionUseCase) AddReaction(userID, targetID int64, targetType, reactionType string) error {
	if f.addReaction != nil {
		return f.addReaction(userID, targetID, targetType, reactionType)
	}
	return nil
}

func (f *fakeReactionUseCase) RemoveReaction(userID, id int64) error {
	if f.removeReaction != nil {
		return f.removeReaction(userID, id)
	}
	return nil
}

func (f *fakeReactionUseCase) GetReactionsByTarget(targetID int64, targetType string) ([]model.Reaction, error) {
	if f.getReactionsByTarget != nil {
		return f.getReactionsByTarget(targetID, targetType)
	}
	return []model.Reaction{}, nil
}

func newTestHandler(
	user authUserPort,
	board port.BoardUseCase,
	post port.PostUseCase,
	comment port.CommentUseCase,
	reaction port.ReactionUseCase,
) http.Handler {
	tokenProvider := auth.NewJwtTokenProvider("test-secret")
	testCache = cacheInMemory.NewInMemoryCache()
	sessionUseCase := service.NewSessionService(user, tokenProvider, testCache)
	return NewHTTPServer(":0", HTTPDependencies{
		SessionUseCase:  sessionUseCase,
		UserUseCase:     user,
		BoardUseCase:    board,
		PostUseCase:     post,
		CommentUseCase:  comment,
		ReactionUseCase: reaction,
	}).Handler
}

func doJSONRequest(t *testing.T, handler http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, apiV1Prefix+path, &buf)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func doJSONRequestWithAuth(t *testing.T, handler http.Handler, method, path string, body any, userID int64) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	token, err := auth.NewJwtTokenProvider("test-secret").IdToToken(userID)
	require.NoError(t, err)
	require.NotNil(t, testCache)
	testCache.Set(token, userID)

	req := httptest.NewRequest(method, apiV1Prefix+path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func TestHTTP_UserSignUp_MethodNotAllowed(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/signup", nil)
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHTTP_UserSignUp_Conflict(t *testing.T) {
	user := &fakeUserUseCase{
		signUp: func(username, password string) (string, error) {
			return "", customError.ErrUserAlreadyExists
		},
	}
	handler := newTestHandler(user, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{})

	rr := doJSONRequest(t, handler, http.MethodPost, "/signup", map[string]string{
		"username": "alice",
		"password": "pw",
	})
	assert.Equal(t, http.StatusConflict, rr.Code)
}

func TestHTTP_UserSignUp_BadJSON(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{})

	req := httptest.NewRequest(http.MethodPost, apiV1Prefix+"/signup", bytes.NewBufferString("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHTTP_UserSignUp_UnknownField(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{})

	rr := doJSONRequest(t, handler, http.MethodPost, "/signup", map[string]any{
		"username": "alice",
		"password": "pw",
		"extra":    "unknown",
	})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHTTP_UserDeleteMe_Unauthorized(t *testing.T) {
	user := &fakeUserUseCase{
		deleteMe: func(userID int64, password string) error {
			return customError.ErrInvalidCredential
		},
	}
	handler := newTestHandler(user, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{})

	rr := doJSONRequestWithAuth(t, handler, http.MethodDelete, "/users/me", map[string]string{
		"password": "wrong",
	}, 1)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestHTTP_BoardCreate_Forbidden(t *testing.T) {
	board := &fakeBoardUseCase{
		createBoard: func(userID int64, name, description string) (int64, error) {
			return 0, customError.ErrForbidden
		},
	}
	handler := newTestHandler(&fakeUserUseCase{}, board, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{})

	rr := doJSONRequestWithAuth(t, handler, http.MethodPost, "/boards", map[string]any{
		"name":        "free",
		"description": "desc",
	}, 2)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestHTTP_BoardGet_BadLimit(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/boards?limit=bad", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHTTP_BoardGet_BadOffset(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/boards?last_id=bad", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHTTP_BoardWithID_InvalidBoardID(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{})

	rr := doJSONRequestWithAuth(t, handler, http.MethodPut, "/boards/abc", map[string]any{
		"name": "free",
	}, 1)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHTTP_BoardWithID_UnauthorizedBeforeValidation(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{})

	rr := doJSONRequest(t, handler, http.MethodPut, "/boards/abc", map[string]any{
		"name": "free",
	})
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestHTTP_PostWithID_InvalidPostID(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/posts/abc", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHTTP_PostWithID_NonPositivePostID(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/posts/0", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	rr = doJSONRequest(t, handler, http.MethodGet, "/posts/-1", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHTTP_ReactionDelete_BadUserID(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{})

	rr := doJSONRequest(t, handler, http.MethodDelete, "/reactions/1?user_id=bad", nil)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestHTTP_PostReactionList_InvalidPostID(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/posts/abc/reactions", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHTTP_CommentReactionList_InvalidCommentID(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/comments/abc/reactions", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHTTP_ReactionWithID_MethodNotAllowed(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/reactions/1", nil)
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHTTP_PostDetail_InternalServerErrorFallback(t *testing.T) {
	post := &fakePostUseCase{
		getPostDetail: func(postID int64) (*model.PostDetail, error) {
			return nil, errors.New("unexpected")
		},
	}
	handler := newTestHandler(&fakeUserUseCase{}, &fakeBoardUseCase{}, post, &fakeCommentUseCase{}, &fakeReactionUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/posts/10", nil)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestHTTP_PostDetail_NotFound(t *testing.T) {
	post := &fakePostUseCase{
		getPostDetail: func(postID int64) (*model.PostDetail, error) {
			return nil, customError.ErrPostNotFound
		},
	}
	handler := newTestHandler(&fakeUserUseCase{}, &fakeBoardUseCase{}, post, &fakeCommentUseCase{}, &fakeReactionUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/posts/10", nil)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHTTP_NotFound(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/unknown", nil)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}
