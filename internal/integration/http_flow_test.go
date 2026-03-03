package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/application"
	"github.com/hoonzinope/go-comu-bin/internal/application/service"
	"github.com/hoonzinope/go-comu-bin/internal/delivery"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/hoonzinope/go-comu-bin/internal/infrastructure/persistence/inmemory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_MainFlow(t *testing.T) {
	server := newIntegrationServer(t)
	defer server.Close()

	adminID := mustLogin(t, server.URL, "admin", "admin")
	boardID := mustCreateBoard(t, server.URL, adminID, "free", "general board")

	mustSignUp(t, server.URL, "alice", "pw")
	userID := mustLogin(t, server.URL, "alice", "pw")

	mustGetBoards(t, server.URL, boardID)

	postID := mustCreatePost(t, server.URL, boardID, userID, "hello", "first post")
	mustGetPost(t, server.URL, postID)
	mustUpdatePost(t, server.URL, postID, userID, "hello-updated", "first post updated")

	commentID := mustCreateComment(t, server.URL, postID, userID, "nice")
	mustGetComments(t, server.URL, postID, commentID)
	mustUpdateComment(t, server.URL, commentID, userID, "nice-updated")

	mustAddReaction(t, server.URL, userID, commentID, "comment", "like")
	commentReactionID := mustGetFirstReactionID(t, server.URL, commentID, "comment")
	mustDeleteReaction(t, server.URL, commentReactionID, userID)
	mustNoReactions(t, server.URL, commentID, "comment")

	mustDeleteComment(t, server.URL, commentID, userID)
	mustNoComments(t, server.URL, postID)

	mustAddReaction(t, server.URL, userID, postID, "post", "like")
	postReactionID := mustGetFirstReactionID(t, server.URL, postID, "post")
	mustDeleteReaction(t, server.URL, postReactionID, userID)
	mustNoReactions(t, server.URL, postID, "post")

	mustDeletePost(t, server.URL, postID, userID)
	mustPostNotAccessible(t, server.URL, postID)

	mustLogout(t, server.URL, "alice")
	mustQuit(t, server.URL, "alice", "pw")
	mustLogout(t, server.URL, "admin")
}

func TestIntegration_ForbiddenScenarios(t *testing.T) {
	server := newIntegrationServer(t)
	defer server.Close()

	adminID := mustLogin(t, server.URL, "admin", "admin")
	boardID := mustCreateBoard(t, server.URL, adminID, "free", "general board")

	mustSignUp(t, server.URL, "alice", "pw")
	aliceID := mustLogin(t, server.URL, "alice", "pw")
	mustSignUp(t, server.URL, "bob", "pw")
	bobID := mustLogin(t, server.URL, "bob", "pw")

	postID := mustCreatePost(t, server.URL, boardID, aliceID, "hello", "first post")
	commentID := mustCreateComment(t, server.URL, postID, aliceID, "nice")
	mustAddReaction(t, server.URL, aliceID, commentID, "comment", "like")
	reactionID := mustGetFirstReactionID(t, server.URL, commentID, "comment")

	assertStatus(t, server.URL, http.MethodPost, "/boards", map[string]any{
		"user_id": bobID, "name": "blocked", "description": "blocked",
	}, http.StatusForbidden)
	assertStatus(t, server.URL, http.MethodPut, fmt.Sprintf("/posts/%d", postID), map[string]any{
		"author_id": bobID, "title": "hack", "content": "hack",
	}, http.StatusForbidden)
	assertStatus(t, server.URL, http.MethodDelete, fmt.Sprintf("/posts/%d?author_id=%d", postID, bobID), nil, http.StatusForbidden)
	assertStatus(t, server.URL, http.MethodPut, fmt.Sprintf("/comments/%d", commentID), map[string]any{
		"author_id": bobID, "content": "hack",
	}, http.StatusForbidden)
	assertStatus(t, server.URL, http.MethodDelete, fmt.Sprintf("/comments/%d?author_id=%d", commentID, bobID), nil, http.StatusForbidden)
	assertStatus(t, server.URL, http.MethodDelete, fmt.Sprintf("/reactions/%d?user_id=%d", reactionID, bobID), nil, http.StatusForbidden)
}

func newIntegrationServer(t *testing.T) *httptest.Server {
	t.Helper()

	repository := application.Repository{
		UserRepository:     inmemory.NewUserRepository(),
		BoardRepository:    inmemory.NewBoardRepository(),
		PostRepository:     inmemory.NewPostRepository(),
		CommentRepository:  inmemory.NewCommentRepository(),
		ReactionRepository: inmemory.NewReactionRepository(),
	}

	admin := &entity.User{}
	admin.NewAdmin("admin", "admin")
	_, err := repository.UserRepository.Save(admin)
	require.NoError(t, err)

	useCases := application.UseCase{
		UserUseCase:     service.NewUserService(repository),
		BoardUseCase:    service.NewBoardService(repository),
		PostUseCase:     service.NewPostService(repository),
		CommentUseCase:  service.NewCommentService(repository),
		ReactionUseCase: service.NewReactionService(repository),
	}

	httpServer := delivery.NewHTTPServer(":0", useCases)
	return httptest.NewServer(httpServer.Handler)
}

func mustLogin(t *testing.T, baseURL, username, password string) int64 {
	t.Helper()
	body, status := requestJSON(t, baseURL, http.MethodPost, "/users/login", map[string]any{
		"username": username,
		"password": password,
	})
	assert.Equal(t, http.StatusOK, status, "login failed: body=%s", string(body))
	var resp map[string]any
	mustUnmarshal(t, body, &resp)
	return int64(resp["user_id"].(float64))
}

func mustSignUp(t *testing.T, baseURL, username, password string) {
	t.Helper()
	body, status := requestJSON(t, baseURL, http.MethodPost, "/users/signup", map[string]any{
		"username": username,
		"password": password,
	})
	assert.Equal(t, http.StatusCreated, status, "signup failed: body=%s", string(body))
}

func mustLogout(t *testing.T, baseURL, username string) {
	t.Helper()
	body, status := requestJSON(t, baseURL, http.MethodPost, "/users/logout", map[string]any{
		"username": username,
	})
	assert.Equal(t, http.StatusOK, status, "logout failed: body=%s", string(body))
}

func mustQuit(t *testing.T, baseURL, username, password string) {
	t.Helper()
	body, status := requestJSON(t, baseURL, http.MethodDelete, "/users/quit", map[string]any{
		"username": username,
		"password": password,
	})
	assert.Equal(t, http.StatusNoContent, status, "quit failed: body=%s", string(body))
}

func mustCreateBoard(t *testing.T, baseURL string, userID int64, name, description string) int64 {
	t.Helper()
	body, status := requestJSON(t, baseURL, http.MethodPost, "/boards", map[string]any{
		"user_id":     userID,
		"name":        name,
		"description": description,
	})
	assert.Equal(t, http.StatusCreated, status, "create board failed: body=%s", string(body))
	var resp map[string]any
	mustUnmarshal(t, body, &resp)
	return int64(resp["id"].(float64))
}

func mustGetBoards(t *testing.T, baseURL string, expectedBoardID int64) {
	t.Helper()
	body, status := requestJSON(t, baseURL, http.MethodGet, "/boards", nil)
	assert.Equal(t, http.StatusOK, status, "get boards failed: body=%s", string(body))
	var resp map[string]any
	mustUnmarshal(t, body, &resp)
	boards := resp["Boards"].([]any)
	require.NotEmpty(t, boards)
	first := boards[0].(map[string]any)
	assert.EqualValues(t, expectedBoardID, int64(first["id"].(float64)))
}

func mustCreatePost(t *testing.T, baseURL string, boardID, authorID int64, title, content string) int64 {
	t.Helper()
	body, status := requestJSON(t, baseURL, http.MethodPost, fmt.Sprintf("/boards/%d/posts", boardID), map[string]any{
		"author_id": authorID,
		"title":     title,
		"content":   content,
	})
	assert.Equal(t, http.StatusCreated, status, "create post failed: body=%s", string(body))
	var resp map[string]any
	mustUnmarshal(t, body, &resp)
	return int64(resp["id"].(float64))
}

func mustGetPost(t *testing.T, baseURL string, postID int64) {
	t.Helper()
	body, status := requestJSON(t, baseURL, http.MethodGet, fmt.Sprintf("/posts/%d", postID), nil)
	assert.Equal(t, http.StatusOK, status, "get post failed: body=%s", string(body))
	var resp map[string]any
	mustUnmarshal(t, body, &resp)
	post := resp["Post"].(map[string]any)
	assert.EqualValues(t, postID, int64(post["id"].(float64)))
}

func mustUpdatePost(t *testing.T, baseURL string, postID, authorID int64, title, content string) {
	t.Helper()
	body, status := requestJSON(t, baseURL, http.MethodPut, fmt.Sprintf("/posts/%d", postID), map[string]any{
		"author_id": authorID,
		"title":     title,
		"content":   content,
	})
	assert.Equal(t, http.StatusNoContent, status, "update post failed: body=%s", string(body))
}

func mustDeletePost(t *testing.T, baseURL string, postID, authorID int64) {
	t.Helper()
	body, status := requestJSON(t, baseURL, http.MethodDelete, fmt.Sprintf("/posts/%d?author_id=%d", postID, authorID), nil)
	assert.Equal(t, http.StatusNoContent, status, "delete post failed: body=%s", string(body))
}

func mustCreateComment(t *testing.T, baseURL string, postID, authorID int64, content string) int64 {
	t.Helper()
	body, status := requestJSON(t, baseURL, http.MethodPost, fmt.Sprintf("/posts/%d/comments", postID), map[string]any{
		"author_id": authorID,
		"content":   content,
	})
	assert.Equal(t, http.StatusCreated, status, "create comment failed: body=%s", string(body))
	var resp map[string]any
	mustUnmarshal(t, body, &resp)
	return int64(resp["id"].(float64))
}

func mustGetComments(t *testing.T, baseURL string, postID, expectedCommentID int64) {
	t.Helper()
	body, status := requestJSON(t, baseURL, http.MethodGet, fmt.Sprintf("/posts/%d/comments", postID), nil)
	assert.Equal(t, http.StatusOK, status, "get comments failed: body=%s", string(body))
	var resp map[string]any
	mustUnmarshal(t, body, &resp)
	comments := resp["Comments"].([]any)
	require.NotEmpty(t, comments)
	first := comments[0].(map[string]any)
	assert.EqualValues(t, expectedCommentID, int64(first["id"].(float64)))
}

func mustUpdateComment(t *testing.T, baseURL string, commentID, authorID int64, content string) {
	t.Helper()
	body, status := requestJSON(t, baseURL, http.MethodPut, fmt.Sprintf("/comments/%d", commentID), map[string]any{
		"author_id": authorID,
		"content":   content,
	})
	assert.Equal(t, http.StatusNoContent, status, "update comment failed: body=%s", string(body))
}

func mustDeleteComment(t *testing.T, baseURL string, commentID, authorID int64) {
	t.Helper()
	body, status := requestJSON(t, baseURL, http.MethodDelete, fmt.Sprintf("/comments/%d?author_id=%d", commentID, authorID), nil)
	assert.Equal(t, http.StatusNoContent, status, "delete comment failed: body=%s", string(body))
}

func mustAddReaction(t *testing.T, baseURL string, userID, targetID int64, targetType, reactionType string) {
	t.Helper()
	body, status := requestJSON(t, baseURL, http.MethodPost, "/reactions", map[string]any{
		"user_id":       userID,
		"target_id":     targetID,
		"target_type":   targetType,
		"reaction_type": reactionType,
	})
	assert.Equal(t, http.StatusCreated, status, "add reaction failed: body=%s", string(body))
}

func mustGetFirstReactionID(t *testing.T, baseURL string, targetID int64, targetType string) int64 {
	t.Helper()
	body, status := requestJSON(t, baseURL, http.MethodGet, fmt.Sprintf("/reactions?target_id=%d&target_type=%s", targetID, targetType), nil)
	assert.Equal(t, http.StatusOK, status, "get reactions failed: body=%s", string(body))
	var resp []map[string]any
	mustUnmarshal(t, body, &resp)
	require.NotEmpty(t, resp)
	return int64(resp[0]["id"].(float64))
}

func mustDeleteReaction(t *testing.T, baseURL string, reactionID, userID int64) {
	t.Helper()
	body, status := requestJSON(t, baseURL, http.MethodDelete, fmt.Sprintf("/reactions/%d?user_id=%d", reactionID, userID), nil)
	assert.Equal(t, http.StatusNoContent, status, "delete reaction failed: body=%s", string(body))
}

func mustNoComments(t *testing.T, baseURL string, postID int64) {
	t.Helper()
	body, status := requestJSON(t, baseURL, http.MethodGet, fmt.Sprintf("/posts/%d/comments", postID), nil)
	assert.Equal(t, http.StatusOK, status, "get comments failed: body=%s", string(body))
	var resp map[string]any
	mustUnmarshal(t, body, &resp)
	rawComments, exists := resp["Comments"]
	if !exists || rawComments == nil {
		return
	}
	comments, ok := rawComments.([]any)
	require.True(t, ok, "unexpected comments payload type: %T", rawComments)
	assert.Empty(t, comments)
}

func mustNoReactions(t *testing.T, baseURL string, targetID int64, targetType string) {
	t.Helper()
	body, status := requestJSON(t, baseURL, http.MethodGet, fmt.Sprintf("/reactions?target_id=%d&target_type=%s", targetID, targetType), nil)
	assert.Equal(t, http.StatusOK, status, "get reactions failed: body=%s", string(body))
	var resp []map[string]any
	mustUnmarshal(t, body, &resp)
	assert.Empty(t, resp)
}

func mustPostNotAccessible(t *testing.T, baseURL string, postID int64) {
	t.Helper()
	body, status := requestJSON(t, baseURL, http.MethodGet, fmt.Sprintf("/posts/%d", postID), nil)
	assert.Equal(t, http.StatusInternalServerError, status, "expected deleted post to be inaccessible(500), body=%s", string(body))
}

func assertStatus(t *testing.T, baseURL, method, path string, body any, expected int) {
	t.Helper()
	respBody, status := requestJSON(t, baseURL, method, path, body)
	assert.Equal(t, expected, status, "path=%s body=%s", path, string(respBody))
}

func requestJSON(t *testing.T, baseURL, method, path string, body any) ([]byte, int) {
	t.Helper()

	var payload io.Reader
	if body != nil {
		buf := &bytes.Buffer{}
		require.NoError(t, json.NewEncoder(buf).Encode(body))
		payload = buf
	}

	req, err := http.NewRequest(method, baseURL+path, payload)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return respBody, resp.StatusCode
}

func mustUnmarshal(t *testing.T, body []byte, dst any) {
	t.Helper()
	require.NoError(t, json.Unmarshal(body, dst), "failed to unmarshal body=%s", string(body))
}
