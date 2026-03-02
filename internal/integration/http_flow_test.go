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
	if _, err := repository.UserRepository.Save(admin); err != nil {
		t.Fatalf("failed to seed admin: %v", err)
	}

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
	if status != http.StatusOK {
		t.Fatalf("login failed: status=%d body=%s", status, string(body))
	}
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
	if status != http.StatusCreated {
		t.Fatalf("signup failed: status=%d body=%s", status, string(body))
	}
}

func mustLogout(t *testing.T, baseURL, username string) {
	t.Helper()
	body, status := requestJSON(t, baseURL, http.MethodPost, "/users/logout", map[string]any{
		"username": username,
	})
	if status != http.StatusOK {
		t.Fatalf("logout failed: status=%d body=%s", status, string(body))
	}
}

func mustQuit(t *testing.T, baseURL, username, password string) {
	t.Helper()
	body, status := requestJSON(t, baseURL, http.MethodDelete, "/users/quit", map[string]any{
		"username": username,
		"password": password,
	})
	if status != http.StatusNoContent {
		t.Fatalf("quit failed: status=%d body=%s", status, string(body))
	}
}

func mustCreateBoard(t *testing.T, baseURL string, userID int64, name, description string) int64 {
	t.Helper()
	body, status := requestJSON(t, baseURL, http.MethodPost, "/boards", map[string]any{
		"user_id":     userID,
		"name":        name,
		"description": description,
	})
	if status != http.StatusCreated {
		t.Fatalf("create board failed: status=%d body=%s", status, string(body))
	}
	var resp map[string]any
	mustUnmarshal(t, body, &resp)
	return int64(resp["id"].(float64))
}

func mustGetBoards(t *testing.T, baseURL string, expectedBoardID int64) {
	t.Helper()
	body, status := requestJSON(t, baseURL, http.MethodGet, "/boards", nil)
	if status != http.StatusOK {
		t.Fatalf("get boards failed: status=%d body=%s", status, string(body))
	}
	var resp map[string]any
	mustUnmarshal(t, body, &resp)
	boards := resp["Boards"].([]any)
	if len(boards) == 0 {
		t.Fatal("expected at least one board")
	}
	first := boards[0].(map[string]any)
	if int64(first["id"].(float64)) != expectedBoardID {
		t.Fatalf("unexpected board id: got=%v want=%d", first["id"], expectedBoardID)
	}
}

func mustCreatePost(t *testing.T, baseURL string, boardID, authorID int64, title, content string) int64 {
	t.Helper()
	body, status := requestJSON(t, baseURL, http.MethodPost, fmt.Sprintf("/boards/%d/posts", boardID), map[string]any{
		"author_id": authorID,
		"title":     title,
		"content":   content,
	})
	if status != http.StatusCreated {
		t.Fatalf("create post failed: status=%d body=%s", status, string(body))
	}
	var resp map[string]any
	mustUnmarshal(t, body, &resp)
	return int64(resp["id"].(float64))
}

func mustGetPost(t *testing.T, baseURL string, postID int64) {
	t.Helper()
	body, status := requestJSON(t, baseURL, http.MethodGet, fmt.Sprintf("/posts/%d", postID), nil)
	if status != http.StatusOK {
		t.Fatalf("get post failed: status=%d body=%s", status, string(body))
	}
	var resp map[string]any
	mustUnmarshal(t, body, &resp)
	post := resp["Post"].(map[string]any)
	if int64(post["id"].(float64)) != postID {
		t.Fatalf("unexpected post id: got=%v want=%d", post["id"], postID)
	}
}

func mustUpdatePost(t *testing.T, baseURL string, postID, authorID int64, title, content string) {
	t.Helper()
	body, status := requestJSON(t, baseURL, http.MethodPut, fmt.Sprintf("/posts/%d", postID), map[string]any{
		"author_id": authorID,
		"title":     title,
		"content":   content,
	})
	if status != http.StatusNoContent {
		t.Fatalf("update post failed: status=%d body=%s", status, string(body))
	}
}

func mustDeletePost(t *testing.T, baseURL string, postID, authorID int64) {
	t.Helper()
	body, status := requestJSON(t, baseURL, http.MethodDelete, fmt.Sprintf("/posts/%d?author_id=%d", postID, authorID), nil)
	if status != http.StatusNoContent {
		t.Fatalf("delete post failed: status=%d body=%s", status, string(body))
	}
}

func mustCreateComment(t *testing.T, baseURL string, postID, authorID int64, content string) int64 {
	t.Helper()
	body, status := requestJSON(t, baseURL, http.MethodPost, fmt.Sprintf("/posts/%d/comments", postID), map[string]any{
		"author_id": authorID,
		"content":   content,
	})
	if status != http.StatusCreated {
		t.Fatalf("create comment failed: status=%d body=%s", status, string(body))
	}
	var resp map[string]any
	mustUnmarshal(t, body, &resp)
	return int64(resp["id"].(float64))
}

func mustGetComments(t *testing.T, baseURL string, postID, expectedCommentID int64) {
	t.Helper()
	body, status := requestJSON(t, baseURL, http.MethodGet, fmt.Sprintf("/posts/%d/comments", postID), nil)
	if status != http.StatusOK {
		t.Fatalf("get comments failed: status=%d body=%s", status, string(body))
	}
	var resp map[string]any
	mustUnmarshal(t, body, &resp)
	comments := resp["Comments"].([]any)
	if len(comments) == 0 {
		t.Fatal("expected at least one comment")
	}
	first := comments[0].(map[string]any)
	if int64(first["id"].(float64)) != expectedCommentID {
		t.Fatalf("unexpected comment id: got=%v want=%d", first["id"], expectedCommentID)
	}
}

func mustUpdateComment(t *testing.T, baseURL string, commentID, authorID int64, content string) {
	t.Helper()
	body, status := requestJSON(t, baseURL, http.MethodPut, fmt.Sprintf("/comments/%d", commentID), map[string]any{
		"author_id": authorID,
		"content":   content,
	})
	if status != http.StatusNoContent {
		t.Fatalf("update comment failed: status=%d body=%s", status, string(body))
	}
}

func mustDeleteComment(t *testing.T, baseURL string, commentID, authorID int64) {
	t.Helper()
	body, status := requestJSON(t, baseURL, http.MethodDelete, fmt.Sprintf("/comments/%d?author_id=%d", commentID, authorID), nil)
	if status != http.StatusNoContent {
		t.Fatalf("delete comment failed: status=%d body=%s", status, string(body))
	}
}

func mustAddReaction(t *testing.T, baseURL string, userID, targetID int64, targetType, reactionType string) {
	t.Helper()
	body, status := requestJSON(t, baseURL, http.MethodPost, "/reactions", map[string]any{
		"user_id":       userID,
		"target_id":     targetID,
		"target_type":   targetType,
		"reaction_type": reactionType,
	})
	if status != http.StatusCreated {
		t.Fatalf("add reaction failed: status=%d body=%s", status, string(body))
	}
}

func mustGetFirstReactionID(t *testing.T, baseURL string, targetID int64, targetType string) int64 {
	t.Helper()
	body, status := requestJSON(t, baseURL, http.MethodGet, fmt.Sprintf("/reactions?target_id=%d&target_type=%s", targetID, targetType), nil)
	if status != http.StatusOK {
		t.Fatalf("get reactions failed: status=%d body=%s", status, string(body))
	}
	var resp []map[string]any
	mustUnmarshal(t, body, &resp)
	if len(resp) == 0 {
		t.Fatalf("expected reactions for target_id=%d target_type=%s", targetID, targetType)
	}
	return int64(resp[0]["id"].(float64))
}

func mustDeleteReaction(t *testing.T, baseURL string, reactionID, userID int64) {
	t.Helper()
	body, status := requestJSON(t, baseURL, http.MethodDelete, fmt.Sprintf("/reactions/%d?user_id=%d", reactionID, userID), nil)
	if status != http.StatusNoContent {
		t.Fatalf("delete reaction failed: status=%d body=%s", status, string(body))
	}
}

func mustNoComments(t *testing.T, baseURL string, postID int64) {
	t.Helper()
	body, status := requestJSON(t, baseURL, http.MethodGet, fmt.Sprintf("/posts/%d/comments", postID), nil)
	if status != http.StatusOK {
		t.Fatalf("get comments failed: status=%d body=%s", status, string(body))
	}
	var resp map[string]any
	mustUnmarshal(t, body, &resp)
	rawComments, exists := resp["Comments"]
	if !exists || rawComments == nil {
		return
	}
	comments, ok := rawComments.([]any)
	if !ok {
		t.Fatalf("unexpected comments payload type: %T", rawComments)
	}
	if len(comments) != 0 {
		t.Fatalf("expected zero comments, got %d", len(comments))
	}
}

func mustNoReactions(t *testing.T, baseURL string, targetID int64, targetType string) {
	t.Helper()
	body, status := requestJSON(t, baseURL, http.MethodGet, fmt.Sprintf("/reactions?target_id=%d&target_type=%s", targetID, targetType), nil)
	if status != http.StatusOK {
		t.Fatalf("get reactions failed: status=%d body=%s", status, string(body))
	}
	var resp []map[string]any
	mustUnmarshal(t, body, &resp)
	if len(resp) != 0 {
		t.Fatalf("expected zero reactions, got %d", len(resp))
	}
}

func mustPostNotAccessible(t *testing.T, baseURL string, postID int64) {
	t.Helper()
	body, status := requestJSON(t, baseURL, http.MethodGet, fmt.Sprintf("/posts/%d", postID), nil)
	if status != http.StatusInternalServerError {
		t.Fatalf("expected deleted post to be inaccessible(500), status=%d body=%s", status, string(body))
	}
}

func assertStatus(t *testing.T, baseURL, method, path string, body any, expected int) {
	t.Helper()
	respBody, status := requestJSON(t, baseURL, method, path, body)
	if status != expected {
		t.Fatalf("expected status=%d got=%d path=%s body=%s", expected, status, path, string(respBody))
	}
}

func requestJSON(t *testing.T, baseURL, method, path string, body any) ([]byte, int) {
	t.Helper()

	var payload io.Reader
	if body != nil {
		buf := &bytes.Buffer{}
		if err := json.NewEncoder(buf).Encode(body); err != nil {
			t.Fatalf("failed to encode request body: %v", err)
		}
		payload = buf
	}

	req, err := http.NewRequest(method, baseURL+path, payload)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	return respBody, resp.StatusCode
}

func mustUnmarshal(t *testing.T, body []byte, dst any) {
	t.Helper()
	if err := json.Unmarshal(body, dst); err != nil {
		t.Fatalf("failed to unmarshal body=%s err=%v", string(body), err)
	}
}
