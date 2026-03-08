package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/service"
	"github.com/hoonzinope/go-comu-bin/internal/delivery"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/hoonzinope/go-comu-bin/internal/infrastructure/auth"
	cacheInMemory "github.com/hoonzinope/go-comu-bin/internal/infrastructure/cache/inmemory"
	"github.com/hoonzinope/go-comu-bin/internal/infrastructure/persistence/inmemory"
	"github.com/hoonzinope/go-comu-bin/internal/infrastructure/storage/localfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const apiV1Prefix = "/api/v1"

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

	mustSetCommentReaction(t, server.URL, aliceToken, commentID, "like", http.StatusCreated)
	mustSetCommentReaction(t, server.URL, aliceToken, commentID, "dislike", http.StatusNoContent)
	mustHaveFirstCommentReactionType(t, server.URL, commentID, "dislike")
	mustDeleteCommentReaction(t, server.URL, aliceToken, commentID)
	mustNoCommentReactions(t, server.URL, commentID)

	mustDeleteComment(t, server.URL, aliceToken, commentID)
	mustNoComments(t, server.URL, postID)

	mustSetPostReaction(t, server.URL, aliceToken, postID, "like", http.StatusCreated)
	mustSetPostReaction(t, server.URL, aliceToken, postID, "dislike", http.StatusNoContent)
	mustHaveFirstPostReactionType(t, server.URL, postID, "dislike")
	mustDeletePostReaction(t, server.URL, aliceToken, postID)
	mustNoPostReactions(t, server.URL, postID)

	mustDeletePost(t, server.URL, aliceToken, postID)
	mustPostNotAccessible(t, server.URL, postID)

	mustDeleteMe(t, server.URL, aliceToken, "pw")
	mustLogout(t, server.URL, adminToken)
	assertStatus(t, server.URL, aliceToken, http.MethodPost, "/boards", map[string]any{
		"name":        "after-logout",
		"description": "should fail",
	}, http.StatusUnauthorized)
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
	mustSetCommentReaction(t, server.URL, aliceToken, commentID, "like", http.StatusCreated)

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
	assertStatus(t, server.URL, bobToken, http.MethodDelete, fmt.Sprintf("/comments/%d/reactions/me", commentID), nil, http.StatusNoContent)
}

func newIntegrationServer(t *testing.T) *httptest.Server {
	t.Helper()

	userRepository := inmemory.NewUserRepository()
	boardRepository := inmemory.NewBoardRepository()
	postRepository := inmemory.NewPostRepository()
	commentRepository := inmemory.NewCommentRepository()
	reactionRepository := inmemory.NewReactionRepository()
	attachmentRepository := inmemory.NewAttachmentRepository()
	fileStorage := localfs.NewFileStorage(t.TempDir())

	cache := cacheInMemory.NewInMemoryCache()
	authorizationPolicy := policy.NewRoleAuthorizationPolicy()
	passwordHasher := auth.NewBcryptPasswordHasher(4)
	hashedAdminPassword, err := passwordHasher.Hash("admin")
	require.NoError(t, err)
	admin := entity.NewAdmin("admin", hashedAdminPassword)
	_, err = userRepository.Save(admin)
	require.NoError(t, err)

	userUseCase := service.NewUserService(userRepository, passwordHasher)
	boardUseCase := service.NewBoardService(userRepository, boardRepository, cache, testCachePolicy(), authorizationPolicy)
	postUseCase := service.NewPostService(userRepository, boardRepository, postRepository, commentRepository, reactionRepository, cache, testCachePolicy(), authorizationPolicy)
	commentUseCase := service.NewCommentService(userRepository, postRepository, commentRepository, cache, testCachePolicy(), authorizationPolicy)
	reactionUseCase := service.NewReactionService(userRepository, postRepository, commentRepository, reactionRepository, cache, testCachePolicy())
	attachmentUseCase := service.NewAttachmentService(userRepository, postRepository, attachmentRepository, fileStorage, authorizationPolicy)

	tokenProvider := auth.NewJwtTokenProvider("test-secret")
	sessionRepository := auth.NewCacheSessionRepository(cache)
	sessionUseCase := service.NewSessionService(userUseCase, tokenProvider, sessionRepository)
	accountUseCase := service.NewAccountService(userUseCase, sessionUseCase)
	httpServer := delivery.NewHTTPServer(":0", delivery.HTTPDependencies{
		SessionUseCase:    sessionUseCase,
		UserUseCase:       userUseCase,
		AccountUseCase:    accountUseCase,
		BoardUseCase:      boardUseCase,
		PostUseCase:       postUseCase,
		CommentUseCase:    commentUseCase,
		ReactionUseCase:   reactionUseCase,
		AttachmentUseCase: attachmentUseCase,
	})
	return httptest.NewServer(httpServer.Handler)
}

func testCachePolicy() appcache.Policy {
	return appcache.Policy{
		ListTTLSeconds:   30,
		DetailTTLSeconds: 30,
	}
}

func mustLogin(t *testing.T, baseURL, username, password string) string {
	t.Helper()
	_, status, headers := requestJSON(t, baseURL, "", http.MethodPost, "/auth/login", map[string]any{
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
	body, status, _ := requestJSON(t, baseURL, "", http.MethodPost, "/signup", map[string]any{
		"username": username,
		"password": password,
	})
	assert.Equal(t, http.StatusCreated, status, "signup failed: body=%s", string(body))
}

func mustLogout(t *testing.T, baseURL, token string) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, token, http.MethodPost, "/auth/logout", map[string]any{})
	assert.Equal(t, http.StatusOK, status, "logout failed: body=%s", string(body))
}

func mustDeleteMe(t *testing.T, baseURL, token, password string) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, token, http.MethodDelete, "/users/me", map[string]any{
		"password": password,
	})
	assert.Equal(t, http.StatusNoContent, status, "delete me failed: body=%s", string(body))
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
	boards := resp["boards"].([]any)
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
	post := resp["post"].(map[string]any)
	assert.EqualValues(t, postID, int64(post["id"].(float64)))
	authorUUID, ok := post["author_uuid"].(string)
	require.True(t, ok)
	assert.NotEmpty(t, authorUUID)
	_, hasAuthorID := post["author_id"]
	assert.False(t, hasAuthorID)
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
	comments := resp["comments"].([]any)
	require.NotEmpty(t, comments)
	first := comments[0].(map[string]any)
	assert.EqualValues(t, expectedCommentID, int64(first["id"].(float64)))
	authorUUID, ok := first["author_uuid"].(string)
	require.True(t, ok)
	assert.NotEmpty(t, authorUUID)
	_, hasAuthorID := first["author_id"]
	assert.False(t, hasAuthorID)
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

func mustSetPostReaction(t *testing.T, baseURL, token string, postID int64, reactionType string, expectedStatus int) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, token, http.MethodPut, fmt.Sprintf("/posts/%d/reactions/me", postID), map[string]any{
		"reaction_type": reactionType,
	})
	assert.Equal(t, expectedStatus, status, "set post reaction failed: body=%s", string(body))
}

func mustSetCommentReaction(t *testing.T, baseURL, token string, commentID int64, reactionType string, expectedStatus int) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, token, http.MethodPut, fmt.Sprintf("/comments/%d/reactions/me", commentID), map[string]any{
		"reaction_type": reactionType,
	})
	assert.Equal(t, expectedStatus, status, "set comment reaction failed: body=%s", string(body))
}

func mustHaveFirstPostReactionType(t *testing.T, baseURL string, postID int64, expectedType string) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, "", http.MethodGet, fmt.Sprintf("/posts/%d/reactions", postID), nil)
	assert.Equal(t, http.StatusOK, status, "get post reactions failed: body=%s", string(body))
	var resp []map[string]any
	mustUnmarshal(t, body, &resp)
	require.NotEmpty(t, resp)
	assert.Equal(t, expectedType, resp[0]["type"])
	userUUID, ok := resp[0]["user_uuid"].(string)
	require.True(t, ok)
	assert.NotEmpty(t, userUUID)
	_, hasUserID := resp[0]["user_id"]
	assert.False(t, hasUserID)
}

func mustHaveFirstCommentReactionType(t *testing.T, baseURL string, commentID int64, expectedType string) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, "", http.MethodGet, fmt.Sprintf("/comments/%d/reactions", commentID), nil)
	assert.Equal(t, http.StatusOK, status, "get comment reactions failed: body=%s", string(body))
	var resp []map[string]any
	mustUnmarshal(t, body, &resp)
	require.NotEmpty(t, resp)
	assert.Equal(t, expectedType, resp[0]["type"])
	userUUID, ok := resp[0]["user_uuid"].(string)
	require.True(t, ok)
	assert.NotEmpty(t, userUUID)
	_, hasUserID := resp[0]["user_id"]
	assert.False(t, hasUserID)
}

func mustDeletePostReaction(t *testing.T, baseURL, token string, postID int64) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, token, http.MethodDelete, fmt.Sprintf("/posts/%d/reactions/me", postID), nil)
	assert.Equal(t, http.StatusNoContent, status, "delete post reaction failed: body=%s", string(body))
}

func mustDeleteCommentReaction(t *testing.T, baseURL, token string, commentID int64) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, token, http.MethodDelete, fmt.Sprintf("/comments/%d/reactions/me", commentID), nil)
	assert.Equal(t, http.StatusNoContent, status, "delete comment reaction failed: body=%s", string(body))
}

func mustNoComments(t *testing.T, baseURL string, postID int64) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, "", http.MethodGet, fmt.Sprintf("/posts/%d/comments", postID), nil)
	assert.Equal(t, http.StatusOK, status, "get comments failed: body=%s", string(body))
	var resp map[string]any
	mustUnmarshal(t, body, &resp)
	rawComments, exists := resp["comments"]
	if !exists || rawComments == nil {
		return
	}
	comments, ok := rawComments.([]any)
	require.True(t, ok, "unexpected comments payload type: %T", rawComments)
	assert.Empty(t, comments)
}

func mustNoPostReactions(t *testing.T, baseURL string, postID int64) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, "", http.MethodGet, fmt.Sprintf("/posts/%d/reactions", postID), nil)
	assert.Equal(t, http.StatusOK, status, "get post reactions failed: body=%s", string(body))
	var resp []map[string]any
	mustUnmarshal(t, body, &resp)
	assert.Empty(t, resp)
}

func mustNoCommentReactions(t *testing.T, baseURL string, commentID int64) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, "", http.MethodGet, fmt.Sprintf("/comments/%d/reactions", commentID), nil)
	assert.Equal(t, http.StatusOK, status, "get comment reactions failed: body=%s", string(body))
	var resp []map[string]any
	mustUnmarshal(t, body, &resp)
	assert.Empty(t, resp)
}

func mustPostNotAccessible(t *testing.T, baseURL string, postID int64) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, "", http.MethodGet, fmt.Sprintf("/posts/%d", postID), nil)
	assert.Equal(t, http.StatusNotFound, status, "expected deleted post to be inaccessible(404), body=%s", string(body))
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

	req, err := http.NewRequest(method, baseURL+apiV1Prefix+path, payload)
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
