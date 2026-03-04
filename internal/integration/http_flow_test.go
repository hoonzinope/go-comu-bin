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

	adminToken := mustLogin(t, server.URL, "admin", "admin")
	boardID := mustCreateBoard(t, server.URL, adminToken, "free", "general board")

	mustSignUp(t, server.URL, "alice", "pw")
	aliceToken := mustLogin(t, server.URL, "alice", "pw")

	mustGetBoards(t, server.URL, boardID)

	postID := mustCreatePost(t, server.URL, aliceToken, boardID, "hello", "first post")
	mustGetPost(t, server.URL, postID)
	mustUpdatePost(t, server.URL, aliceToken, postID, "hello-updated", "first post updated")

	commentID := mustCreateComment(t, server.URL, aliceToken, postID, "nice")
	mustGetComments(t, server.URL, postID, commentID)
	mustUpdateComment(t, server.URL, aliceToken, commentID, "nice-updated")

	mustAddReaction(t, server.URL, aliceToken, commentID, "comment", "like")
	commentReactionID := mustGetFirstReactionID(t, server.URL, commentID, "comment")
	mustDeleteReaction(t, server.URL, aliceToken, commentReactionID)
	mustNoReactions(t, server.URL, commentID, "comment")

	mustDeleteComment(t, server.URL, aliceToken, commentID)
	mustNoComments(t, server.URL, postID)

	mustAddReaction(t, server.URL, aliceToken, postID, "post", "like")
	postReactionID := mustGetFirstReactionID(t, server.URL, postID, "post")
	mustDeleteReaction(t, server.URL, aliceToken, postReactionID)
	mustNoReactions(t, server.URL, postID, "post")

	mustDeletePost(t, server.URL, aliceToken, postID)
	mustPostNotAccessible(t, server.URL, postID)

	mustLogout(t, server.URL, adminToken)
	mustLogout(t, server.URL, aliceToken)
	mustQuit(t, server.URL, "alice", "pw")
}

func TestIntegration_ForbiddenScenarios(t *testing.T) {
	server := newIntegrationServer(t)
	defer server.Close()

	adminToken := mustLogin(t, server.URL, "admin", "admin")
	boardID := mustCreateBoard(t, server.URL, adminToken, "free", "general board")

	mustSignUp(t, server.URL, "alice", "pw")
	aliceToken := mustLogin(t, server.URL, "alice", "pw")
	mustSignUp(t, server.URL, "bob", "pw")
	bobToken := mustLogin(t, server.URL, "bob", "pw")

	postID := mustCreatePost(t, server.URL, aliceToken, boardID, "hello", "first post")
	commentID := mustCreateComment(t, server.URL, aliceToken, postID, "nice")
	mustAddReaction(t, server.URL, aliceToken, commentID, "comment", "like")
	reactionID := mustGetFirstReactionID(t, server.URL, commentID, "comment")

	assertStatus(t, server.URL, bobToken, http.MethodPost, "/boards", map[string]any{
		"name": "blocked", "description": "blocked",
	}, http.StatusForbidden)
	assertStatus(t, server.URL, bobToken, http.MethodPut, fmt.Sprintf("/posts/%d", postID), map[string]any{
		"title": "hack", "content": "hack",
	}, http.StatusForbidden)
	assertStatus(t, server.URL, bobToken, http.MethodDelete, fmt.Sprintf("/posts/%d", postID), nil, http.StatusForbidden)
	assertStatus(t, server.URL, bobToken, http.MethodPut, fmt.Sprintf("/comments/%d", commentID), map[string]any{
		"content": "hack",
	}, http.StatusForbidden)
	assertStatus(t, server.URL, bobToken, http.MethodDelete, fmt.Sprintf("/comments/%d", commentID), nil, http.StatusForbidden)
	assertStatus(t, server.URL, bobToken, http.MethodDelete, fmt.Sprintf("/reactions/%d", reactionID), nil, http.StatusForbidden)
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

	admin := entity.NewAdmin("admin", "admin")
	_, err := repository.UserRepository.Save(admin)
	require.NoError(t, err)

	useCases := application.UseCase{
		UserUseCase:     service.NewUserService(repository),
		BoardUseCase:    service.NewBoardService(repository),
		PostUseCase:     service.NewPostService(repository),
		CommentUseCase:  service.NewCommentService(repository),
		ReactionUseCase: service.NewReactionService(repository),
	}

	httpServer := delivery.NewHTTPServer(":0", "test-secret", useCases)
	return httptest.NewServer(httpServer.Handler)
}

func mustLogin(t *testing.T, baseURL, username, password string) string {
	t.Helper()
	_, status, headers := requestJSON(t, baseURL, "", http.MethodPost, "/users/login", map[string]any{
		"username": username,
		"password": password,
	})
	assert.Equal(t, http.StatusOK, status)
	token := headers.Get("Authorization")
	require.NotEmpty(t, token)
	return token
}

func mustSignUp(t *testing.T, baseURL, username, password string) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, "", http.MethodPost, "/users/signup", map[string]any{
		"username": username,
		"password": password,
	})
	assert.Equal(t, http.StatusCreated, status, "signup failed: body=%s", string(body))
}

func mustLogout(t *testing.T, baseURL, token string) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, token, http.MethodPost, "/users/logout", map[string]any{})
	assert.Equal(t, http.StatusOK, status, "logout failed: body=%s", string(body))
}

func mustQuit(t *testing.T, baseURL, username, password string) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, "", http.MethodDelete, "/users/quit", map[string]any{
		"username": username,
		"password": password,
	})
	assert.Equal(t, http.StatusNoContent, status, "quit failed: body=%s", string(body))
}

func mustCreateBoard(t *testing.T, baseURL, token, name, description string) int64 {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, token, http.MethodPost, "/boards", map[string]any{
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
	body, status, _ := requestJSON(t, baseURL, "", http.MethodGet, "/boards", nil)
	assert.Equal(t, http.StatusOK, status, "get boards failed: body=%s", string(body))
	var resp map[string]any
	mustUnmarshal(t, body, &resp)
	boards := resp["Boards"].([]any)
	require.NotEmpty(t, boards)
	first := boards[0].(map[string]any)
	assert.EqualValues(t, expectedBoardID, int64(first["id"].(float64)))
}

func mustCreatePost(t *testing.T, baseURL, token string, boardID int64, title, content string) int64 {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, token, http.MethodPost, fmt.Sprintf("/boards/%d/posts", boardID), map[string]any{
		"title":   title,
		"content": content,
	})
	assert.Equal(t, http.StatusCreated, status, "create post failed: body=%s", string(body))
	var resp map[string]any
	mustUnmarshal(t, body, &resp)
	return int64(resp["id"].(float64))
}

func mustGetPost(t *testing.T, baseURL string, postID int64) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, "", http.MethodGet, fmt.Sprintf("/posts/%d", postID), nil)
	assert.Equal(t, http.StatusOK, status, "get post failed: body=%s", string(body))
	var resp map[string]any
	mustUnmarshal(t, body, &resp)
	post := resp["Post"].(map[string]any)
	assert.EqualValues(t, postID, int64(post["id"].(float64)))
}

func mustUpdatePost(t *testing.T, baseURL, token string, postID int64, title, content string) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, token, http.MethodPut, fmt.Sprintf("/posts/%d", postID), map[string]any{
		"title":   title,
		"content": content,
	})
	assert.Equal(t, http.StatusNoContent, status, "update post failed: body=%s", string(body))
}

func mustDeletePost(t *testing.T, baseURL, token string, postID int64) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, token, http.MethodDelete, fmt.Sprintf("/posts/%d", postID), nil)
	assert.Equal(t, http.StatusNoContent, status, "delete post failed: body=%s", string(body))
}

func mustCreateComment(t *testing.T, baseURL, token string, postID int64, content string) int64 {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, token, http.MethodPost, fmt.Sprintf("/posts/%d/comments", postID), map[string]any{
		"content": content,
	})
	assert.Equal(t, http.StatusCreated, status, "create comment failed: body=%s", string(body))
	var resp map[string]any
	mustUnmarshal(t, body, &resp)
	return int64(resp["id"].(float64))
}

func mustGetComments(t *testing.T, baseURL string, postID, expectedCommentID int64) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, "", http.MethodGet, fmt.Sprintf("/posts/%d/comments", postID), nil)
	assert.Equal(t, http.StatusOK, status, "get comments failed: body=%s", string(body))
	var resp map[string]any
	mustUnmarshal(t, body, &resp)
	comments := resp["Comments"].([]any)
	require.NotEmpty(t, comments)
	first := comments[0].(map[string]any)
	assert.EqualValues(t, expectedCommentID, int64(first["id"].(float64)))
}

func mustUpdateComment(t *testing.T, baseURL, token string, commentID int64, content string) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, token, http.MethodPut, fmt.Sprintf("/comments/%d", commentID), map[string]any{
		"content": content,
	})
	assert.Equal(t, http.StatusNoContent, status, "update comment failed: body=%s", string(body))
}

func mustDeleteComment(t *testing.T, baseURL, token string, commentID int64) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, token, http.MethodDelete, fmt.Sprintf("/comments/%d", commentID), nil)
	assert.Equal(t, http.StatusNoContent, status, "delete comment failed: body=%s", string(body))
}

func mustAddReaction(t *testing.T, baseURL, token string, targetID int64, targetType, reactionType string) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, token, http.MethodPost, "/reactions", map[string]any{
		"target_id":     targetID,
		"target_type":   targetType,
		"reaction_type": reactionType,
	})
	assert.Equal(t, http.StatusCreated, status, "add reaction failed: body=%s", string(body))
}

func mustGetFirstReactionID(t *testing.T, baseURL string, targetID int64, targetType string) int64 {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, "", http.MethodGet, fmt.Sprintf("/reactions?target_id=%d&target_type=%s", targetID, targetType), nil)
	assert.Equal(t, http.StatusOK, status, "get reactions failed: body=%s", string(body))
	var resp []map[string]any
	mustUnmarshal(t, body, &resp)
	require.NotEmpty(t, resp)
	return int64(resp[0]["id"].(float64))
}

func mustDeleteReaction(t *testing.T, baseURL, token string, reactionID int64) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, token, http.MethodDelete, fmt.Sprintf("/reactions/%d", reactionID), nil)
	assert.Equal(t, http.StatusNoContent, status, "delete reaction failed: body=%s", string(body))
}

func mustNoComments(t *testing.T, baseURL string, postID int64) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, "", http.MethodGet, fmt.Sprintf("/posts/%d/comments", postID), nil)
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
	body, status, _ := requestJSON(t, baseURL, "", http.MethodGet, fmt.Sprintf("/reactions?target_id=%d&target_type=%s", targetID, targetType), nil)
	assert.Equal(t, http.StatusOK, status, "get reactions failed: body=%s", string(body))
	var resp []map[string]any
	mustUnmarshal(t, body, &resp)
	assert.Empty(t, resp)
}

func mustPostNotAccessible(t *testing.T, baseURL string, postID int64) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, "", http.MethodGet, fmt.Sprintf("/posts/%d", postID), nil)
	assert.Equal(t, http.StatusInternalServerError, status, "expected deleted post to be inaccessible(500), body=%s", string(body))
}

func assertStatus(t *testing.T, baseURL, token, method, path string, body any, expected int) {
	t.Helper()
	respBody, status, _ := requestJSON(t, baseURL, token, method, path, body)
	assert.Equal(t, expected, status, "path=%s body=%s", path, string(respBody))
}

func requestJSON(t *testing.T, baseURL, token, method, path string, body any) ([]byte, int, http.Header) {
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
	if token != "" {
		req.Header.Set("Authorization", token)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return respBody, resp.StatusCode, resp.Header
}

func mustUnmarshal(t *testing.T, body []byte, dst any) {
	t.Helper()
	require.NoError(t, json.Unmarshal(body, dst), "failed to unmarshal body=%s", string(body))
}
