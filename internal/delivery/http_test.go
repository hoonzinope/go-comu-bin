package delivery

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/application"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/dto"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type fakeUserUseCase struct {
	signUp func(username, password string) (string, error)
	quit   func(username, password string) error
	login  func(username, password string) (int64, error)
	logout func(username string) error
}

func (f *fakeUserUseCase) SignUp(username, password string) (string, error) {
	if f.signUp != nil {
		return f.signUp(username, password)
	}
	return "ok", nil
}

func (f *fakeUserUseCase) Quit(username, password string) error {
	if f.quit != nil {
		return f.quit(username, password)
	}
	return nil
}

func (f *fakeUserUseCase) Login(username, password string) (int64, error) {
	if f.login != nil {
		return f.login(username, password)
	}
	return 1, nil
}

func (f *fakeUserUseCase) Logout(username string) error {
	if f.logout != nil {
		return f.logout(username)
	}
	return nil
}

type fakeBoardUseCase struct {
	getBoards   func(limit, offset int) (*dto.BoardList, error)
	createBoard func(userID int64, name, description string) (int64, error)
	updateBoard func(id, userID int64, name, description string) error
	deleteBoard func(id, userID int64) error
}

func (f *fakeBoardUseCase) GetBoards(limit, offset int) (*dto.BoardList, error) {
	if f.getBoards != nil {
		return f.getBoards(limit, offset)
	}
	return &dto.BoardList{}, nil
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
	getPostsList  func(boardID int64, limit, offset int) (*dto.PostList, error)
	getPostDetail func(postID int64) (*dto.PostDetail, error)
	updatePost    func(id, authorID int64, title, content string) error
	deletePost    func(id, authorID int64) error
}

func (f *fakePostUseCase) CreatePost(title, content string, authorID, boardID int64) (int64, error) {
	if f.createPost != nil {
		return f.createPost(title, content, authorID, boardID)
	}
	return 1, nil
}

func (f *fakePostUseCase) GetPostsList(boardID int64, limit, offset int) (*dto.PostList, error) {
	if f.getPostsList != nil {
		return f.getPostsList(boardID, limit, offset)
	}
	return &dto.PostList{}, nil
}

func (f *fakePostUseCase) GetPostDetail(postID int64) (*dto.PostDetail, error) {
	if f.getPostDetail != nil {
		return f.getPostDetail(postID)
	}
	return &dto.PostDetail{}, nil
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
	getCommentsByPost func(postID int64, limit, offset int) (*dto.CommentList, error)
	updateComment     func(id, authorID int64, content string) error
	deleteComment     func(id, authorID int64) error
}

func (f *fakeCommentUseCase) CreateComment(content string, authorID, postID int64) (int64, error) {
	if f.createComment != nil {
		return f.createComment(content, authorID, postID)
	}
	return 1, nil
}

func (f *fakeCommentUseCase) GetCommentsByPost(postID int64, limit, offset int) (*dto.CommentList, error) {
	if f.getCommentsByPost != nil {
		return f.getCommentsByPost(postID, limit, offset)
	}
	return &dto.CommentList{}, nil
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
	getReactionsByTarget func(targetID int64, targetType string) ([]*entity.Reaction, error)
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

func (f *fakeReactionUseCase) GetReactionsByTarget(targetID int64, targetType string) ([]*entity.Reaction, error) {
	if f.getReactionsByTarget != nil {
		return f.getReactionsByTarget(targetID, targetType)
	}
	return []*entity.Reaction{}, nil
}

func newTestHandler(
	user application.UserUseCase,
	board application.BoardUseCase,
	post application.PostUseCase,
	comment application.CommentUseCase,
	reaction application.ReactionUseCase,
) http.Handler {
	uc := application.UseCase{
		UserUseCase:     user,
		BoardUseCase:    board,
		PostUseCase:     post,
		CommentUseCase:  comment,
		ReactionUseCase: reaction,
	}
	return NewHTTPServer(":0", uc).Handler
}

func doJSONRequest(t *testing.T, handler http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("failed to encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func TestHTTP_UserSignUp_MethodNotAllowed(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/users/signup", nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestHTTP_UserSignUp_Conflict(t *testing.T) {
	user := &fakeUserUseCase{
		signUp: func(username, password string) (string, error) {
			return "", customError.ErrUserAlreadyExists
		},
	}
	handler := newTestHandler(user, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{})

	rr := doJSONRequest(t, handler, http.MethodPost, "/users/signup", map[string]string{
		"username": "alice",
		"password": "pw",
	})
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rr.Code)
	}
}

func TestHTTP_UserSignUp_BadJSON(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{})

	req := httptest.NewRequest(http.MethodPost, "/users/signup", bytes.NewBufferString("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHTTP_UserSignUp_UnknownField(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{})

	rr := doJSONRequest(t, handler, http.MethodPost, "/users/signup", map[string]any{
		"username": "alice",
		"password": "pw",
		"extra":    "unknown",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHTTP_UserQuit_Unauthorized(t *testing.T) {
	user := &fakeUserUseCase{
		quit: func(username, password string) error {
			return customError.ErrInvalidCredential
		},
	}
	handler := newTestHandler(user, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{})

	rr := doJSONRequest(t, handler, http.MethodDelete, "/users/quit", map[string]string{
		"username": "alice",
		"password": "wrong",
	})
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestHTTP_BoardCreate_Forbidden(t *testing.T) {
	board := &fakeBoardUseCase{
		createBoard: func(userID int64, name, description string) (int64, error) {
			return 0, customError.ErrForbidden
		},
	}
	handler := newTestHandler(&fakeUserUseCase{}, board, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{})

	rr := doJSONRequest(t, handler, http.MethodPost, "/boards", map[string]any{
		"user_id":     2,
		"name":        "free",
		"description": "desc",
	})
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestHTTP_BoardGet_BadLimit(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/boards?limit=bad", nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHTTP_BoardGet_BadOffset(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/boards?offset=bad", nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHTTP_BoardWithID_InvalidBoardID(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{})

	rr := doJSONRequest(t, handler, http.MethodPut, "/boards/abc", map[string]any{
		"user_id": 1,
		"name":    "free",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHTTP_PostWithID_InvalidPostID(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/posts/abc", nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHTTP_ReactionDelete_BadUserID(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{})

	rr := doJSONRequest(t, handler, http.MethodDelete, "/reactions/1?user_id=bad", nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHTTP_ReactionList_MissingTargetType(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/reactions?target_id=1", nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHTTP_ReactionList_MissingTargetID(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/reactions?target_type=post", nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHTTP_ReactionWithID_MethodNotAllowed(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/reactions/1", nil)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestHTTP_PostDetail_InternalServerErrorFallback(t *testing.T) {
	post := &fakePostUseCase{
		getPostDetail: func(postID int64) (*dto.PostDetail, error) {
			return nil, errors.New("unexpected")
		},
	}
	handler := newTestHandler(&fakeUserUseCase{}, &fakeBoardUseCase{}, post, &fakeCommentUseCase{}, &fakeReactionUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/posts/10", nil)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestHTTP_NotFound(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/unknown", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}
