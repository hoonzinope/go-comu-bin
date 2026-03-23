package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
	appevent "github.com/hoonzinope/go-comu-bin/internal/application/event"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/service"
	"github.com/hoonzinope/go-comu-bin/internal/delivery"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/hoonzinope/go-comu-bin/internal/infrastructure/auth"
	cacheInMemory "github.com/hoonzinope/go-comu-bin/internal/infrastructure/cache/inmemory"
	eventOutbox "github.com/hoonzinope/go-comu-bin/internal/infrastructure/event/outbox"
	noopmail "github.com/hoonzinope/go-comu-bin/internal/infrastructure/mail/noop"
	"github.com/hoonzinope/go-comu-bin/internal/infrastructure/persistence/inmemory"
	"github.com/hoonzinope/go-comu-bin/internal/infrastructure/storage/localfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const apiV1Prefix = "/api/v1"

type recordingEmailVerificationMailSender struct {
	lastTokenByEmail map[string]string
}

func newRecordingEmailVerificationMailSender() *recordingEmailVerificationMailSender {
	return &recordingEmailVerificationMailSender{lastTokenByEmail: map[string]string{}}
}

func (s *recordingEmailVerificationMailSender) SendEmailVerification(ctx context.Context, email, token string, expiresAt time.Time) error {
	_ = ctx
	_ = expiresAt
	s.lastTokenByEmail[email] = token
	return nil
}

func (s *recordingEmailVerificationMailSender) tokenFor(email string) string {
	return s.lastTokenByEmail[email]
}

type integrationServer struct {
	*httptest.Server
	verificationMailer *recordingEmailVerificationMailSender
}

func TestIntegration_MainFlow(t *testing.T) {
	server := newIntegrationServer(t)
	defer server.Close()

	adminToken := mustLogin(t, server.URL, "admin", "admin")
	boardUUID := mustCreateBoard(t, server.URL, adminToken, "free", "general board")

	mustSignUp(t, server, "alice", "pw")
	aliceToken := mustLogin(t, server.URL, "alice", "pw")

	mustGetBoards(t, server.URL, boardUUID)

	postUUID := mustCreatePost(t, server.URL, aliceToken, boardUUID, "hello", "first post")
	mustGetPost(t, server.URL, postUUID)
	mustUpdatePost(t, server.URL, aliceToken, postUUID, "hello-updated", "first post updated")

	commentUUID := mustCreateComment(t, server.URL, aliceToken, postUUID, "nice")
	mustGetComments(t, server.URL, postUUID, commentUUID)
	mustUpdateComment(t, server.URL, aliceToken, commentUUID, "nice-updated")

	mustSetCommentReaction(t, server.URL, aliceToken, commentUUID, "like", http.StatusCreated)
	mustSetCommentReaction(t, server.URL, aliceToken, commentUUID, "dislike", http.StatusNoContent)
	mustHaveFirstCommentReactionType(t, server.URL, commentUUID, "dislike")
	mustDeleteCommentReaction(t, server.URL, aliceToken, commentUUID)
	mustNoCommentReactions(t, server.URL, commentUUID)

	mustDeleteComment(t, server.URL, aliceToken, commentUUID)
	mustNoComments(t, server.URL, postUUID)

	mustSetPostReaction(t, server.URL, aliceToken, postUUID, "like", http.StatusCreated)
	mustSetPostReaction(t, server.URL, aliceToken, postUUID, "dislike", http.StatusNoContent)
	mustHaveFirstPostReactionType(t, server.URL, postUUID, "dislike")
	mustDeletePostReaction(t, server.URL, aliceToken, postUUID)
	mustNoPostReactions(t, server.URL, postUUID)

	mustDeletePost(t, server.URL, aliceToken, postUUID)
	mustPostNotAccessible(t, server.URL, postUUID)

	mustDeleteMe(t, server.URL, aliceToken, "pw")
	mustLogout(t, server.URL, adminToken)
	assertStatus(t, server.URL, aliceToken, http.MethodPost, "/boards", map[string]any{
		"name":        "after-logout",
		"description": "should fail",
	}, http.StatusUnauthorized)
}

func TestIntegration_DeleteParentCommentAlsoHidesReply(t *testing.T) {
	server := newIntegrationServer(t)
	defer server.Close()

	adminToken := mustLogin(t, server.URL, "admin", "admin")
	boardUUID := mustCreateBoard(t, server.URL, adminToken, "free", "general board")

	mustSignUp(t, server, "alice", "pw")
	aliceToken := mustLogin(t, server.URL, "alice", "pw")

	postUUID := mustCreatePost(t, server.URL, aliceToken, boardUUID, "hello", "first post")
	parentUUID := mustCreateComment(t, server.URL, aliceToken, postUUID, "parent")
	_ = mustCreateReplyComment(t, server.URL, aliceToken, postUUID, parentUUID, "reply")

	mustDeleteComment(t, server.URL, aliceToken, parentUUID)
	mustHaveDeletedParentAndVisibleReply(t, server.URL, postUUID, parentUUID)
}

func TestIntegration_ForbiddenScenarios(t *testing.T) {
	server := newIntegrationServer(t)
	defer server.Close()

	adminToken := mustLogin(t, server.URL, "admin", "admin")
	boardUUID := mustCreateBoard(t, server.URL, adminToken, "free", "general board")

	mustSignUp(t, server, "alice", "pw")
	aliceToken := mustLogin(t, server.URL, "alice", "pw")
	mustSignUp(t, server, "bob", "pw")
	bobToken := mustLogin(t, server.URL, "bob", "pw")

	postUUID := mustCreatePost(t, server.URL, aliceToken, boardUUID, "hello", "first post")
	commentUUID := mustCreateComment(t, server.URL, aliceToken, postUUID, "nice")
	mustSetCommentReaction(t, server.URL, aliceToken, commentUUID, "like", http.StatusCreated)

	assertStatus(t, server.URL, bobToken, http.MethodPost, "/boards", map[string]any{
		"name": "blocked", "description": "blocked",
	}, http.StatusForbidden)
	assertStatus(t, server.URL, bobToken, http.MethodPut, fmt.Sprintf("/posts/%s", postUUID), map[string]any{
		"title": "hack", "content": "hack",
	}, http.StatusForbidden)
	assertStatus(t, server.URL, bobToken, http.MethodDelete, fmt.Sprintf("/posts/%s", postUUID), nil, http.StatusForbidden)
	assertStatus(t, server.URL, bobToken, http.MethodPut, fmt.Sprintf("/comments/%s", commentUUID), map[string]any{
		"content": "hack",
	}, http.StatusForbidden)
	assertStatus(t, server.URL, bobToken, http.MethodDelete, fmt.Sprintf("/comments/%s", commentUUID), nil, http.StatusForbidden)
	assertStatus(t, server.URL, bobToken, http.MethodDelete, fmt.Sprintf("/comments/%s/reactions/me", commentUUID), nil, http.StatusNoContent)
}

func TestIntegration_GuestUpgrade_RotatesBearerToken(t *testing.T) {
	server := newIntegrationServer(t)
	defer server.Close()

	guestToken := mustIssueGuest(t, server.URL)
	newToken := mustUpgradeGuest(t, server, guestToken, "guest-upgraded", "guest-upgraded@example.com", "pw")

	require.NotEmpty(t, newToken)
	assert.NotEqual(t, guestToken, newToken)

	assertStatus(t, server.URL, guestToken, http.MethodPost, "/auth/logout", map[string]any{}, http.StatusUnauthorized)
	assertStatus(t, server.URL, newToken, http.MethodPost, "/auth/logout", map[string]any{}, http.StatusOK)
}

func newIntegrationServer(t *testing.T) *integrationServer {
	t.Helper()

	userRepository := inmemory.NewUserRepository()
	boardRepository := inmemory.NewBoardRepository()
	tagRepository := inmemory.NewTagRepository()
	postTagRepository := inmemory.NewPostTagRepository()
	postRepository := inmemory.NewPostRepository(tagRepository, postTagRepository)
	postSearchStore := inmemory.NewPostSearchStore(postRepository, tagRepository, postTagRepository)
	commentRepository := inmemory.NewCommentRepository()
	reactionRepository := inmemory.NewReactionRepository()
	attachmentRepository := inmemory.NewAttachmentRepository()
	reportRepository := inmemory.NewReportRepository()
	notificationRepository := inmemory.NewNotificationRepository()
	emailVerificationRepository := inmemory.NewEmailVerificationTokenRepository()
	passwordResetRepository := inmemory.NewPasswordResetTokenRepository()
	outboxRepository := inmemory.NewOutboxRepository()
	fileStorage := localfs.NewFileStorage(t.TempDir())

	cache := cacheInMemory.NewInMemoryCache()
	authorizationPolicy := policy.NewRoleAuthorizationPolicy()
	unitOfWork := inmemory.NewUnitOfWork(userRepository, boardRepository, postRepository, tagRepository, postTagRepository, commentRepository, reactionRepository, attachmentRepository, reportRepository, notificationRepository, emailVerificationRepository, passwordResetRepository, outboxRepository)
	appLogger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	eventSerializer := appevent.NewJSONEventSerializer()
	outboxRelay := eventOutbox.NewRelay(outboxRepository, eventSerializer, appLogger, eventOutbox.RelayConfig{
		WorkerCount:  1,
		BatchSize:    64,
		PollInterval: 5 * time.Millisecond,
		MaxAttempts:  5,
		BaseBackoff:  10 * time.Millisecond,
	})
	cacheInvalidationHandler := appevent.NewCacheInvalidationHandler(cache, appLogger)
	postSearchIndexHandler := appevent.NewPostSearchIndexHandler(postSearchStore)
	notificationHandler := appevent.NewNotificationHandler(notificationRepository)
	outboxRelay.Subscribe(appevent.EventNameBoardChanged, cacheInvalidationHandler)
	outboxRelay.Subscribe(appevent.EventNamePostChanged, cacheInvalidationHandler)
	outboxRelay.Subscribe(appevent.EventNamePostChanged, postSearchIndexHandler)
	outboxRelay.Subscribe(appevent.EventNameCommentChanged, cacheInvalidationHandler)
	outboxRelay.Subscribe(appevent.EventNameReactionChanged, cacheInvalidationHandler)
	outboxRelay.Subscribe(appevent.EventNameAttachmentChanged, cacheInvalidationHandler)
	outboxRelay.Subscribe(appevent.EventNameReportChanged, cacheInvalidationHandler)
	outboxRelay.Subscribe(appevent.EventNameNotificationTriggered, notificationHandler)
	require.NoError(t, postSearchStore.RebuildAll(context.Background()))
	relayCtx, relayCancel := context.WithCancel(context.Background())
	outboxRelay.Start(relayCtx)
	t.Cleanup(func() {
		relayCancel()
		outboxRelay.Wait()
	})
	passwordHasher := auth.NewBcryptPasswordHasher(4)
	verificationMailer := newRecordingEmailVerificationMailSender()
	hashedAdminPassword, err := passwordHasher.Hash("admin")
	require.NoError(t, err)
	admin := entity.NewAdmin("admin", hashedAdminPassword)
	_, err = userRepository.Save(context.Background(), admin)
	require.NoError(t, err)

	userUseCase := service.NewUserServiceWithEmailVerification(userRepository, passwordHasher, unitOfWork, emailVerificationRepository, auth.NewEmailVerificationTokenIssuer(), verificationMailer, 30*time.Minute)
	boardUseCase := service.NewBoardServiceWithActionDispatcher(userRepository, boardRepository, postRepository, unitOfWork, cache, nil, testCachePolicy(), authorizationPolicy)
	postUseCase := service.NewPostServiceWithActionDispatcher(userRepository, boardRepository, postRepository, postSearchStore, tagRepository, postTagRepository, attachmentRepository, commentRepository, reactionRepository, unitOfWork, cache, nil, testCachePolicy(), authorizationPolicy)
	commentUseCase := service.NewCommentServiceWithActionDispatcher(userRepository, boardRepository, postRepository, commentRepository, reactionRepository, unitOfWork, cache, nil, testCachePolicy(), authorizationPolicy)
	notificationUseCase := service.NewNotificationService(userRepository, postRepository, commentRepository, notificationRepository)
	reactionUseCase := service.NewReactionServiceWithActionDispatcher(userRepository, boardRepository, postRepository, commentRepository, reactionRepository, unitOfWork, cache, nil, testCachePolicy())
	reportUseCase := service.NewReportServiceWithActionDispatcher(userRepository, postRepository, commentRepository, reportRepository, unitOfWork, nil, authorizationPolicy)
	outboxAdminUseCase := service.NewOutboxAdminService(userRepository, outboxRepository, authorizationPolicy)
	attachmentUseCase := service.NewAttachmentServiceWithActionDispatcher(userRepository, boardRepository, postRepository, attachmentRepository, unitOfWork, fileStorage, cache, nil, 10<<20, service.ImageOptimizationConfig{Enabled: true, JPEGQuality: 82}, authorizationPolicy)

	tokenProvider := auth.NewJwtTokenProvider("test-secret")
	sessionRepository := auth.NewCacheSessionRepository(cache)
	sessionUseCase := service.NewSessionService(userUseCase, userUseCase, userRepository, tokenProvider, sessionRepository)
	accountUseCase := service.NewAccountServiceWithGuestUpgrade(
		userUseCase,
		sessionUseCase,
		userRepository,
		unitOfWork,
		passwordHasher,
		tokenProvider,
		sessionRepository,
		emailVerificationRepository,
		auth.NewEmailVerificationTokenIssuer(),
		verificationMailer,
		30*time.Minute,
		passwordResetRepository,
		auth.NewPasswordResetTokenIssuer(),
		noopmail.NewPasswordResetMailSender(),
		30*time.Minute,
	)
	httpServer := delivery.NewHTTPServer(":0", delivery.HTTPDependencies{
		SessionUseCase:      sessionUseCase,
		UserUseCase:         userUseCase,
		AdminAuthorizer:     userUseCase,
		AccountUseCase:      accountUseCase,
		BoardUseCase:        boardUseCase,
		PostUseCase:         postUseCase,
		CommentUseCase:      commentUseCase,
		NotificationUseCase: notificationUseCase,
		ReactionUseCase:     reactionUseCase,
		AttachmentUseCase:   attachmentUseCase,
		ReportUseCase:       reportUseCase,
		OutboxAdminUseCase:  outboxAdminUseCase,
	})
	return &integrationServer{
		Server:             httptest.NewServer(httpServer.Handler),
		verificationMailer: verificationMailer,
	}
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

func mustSignUp(t *testing.T, server *integrationServer, username, password string) {
	t.Helper()
	baseURL := server.URL
	body, status, _ := requestJSON(t, baseURL, "", http.MethodPost, "/signup", map[string]any{
		"username": username,
		"email":    fmt.Sprintf("%s@example.com", username),
		"password": password,
	})
	assert.Equal(t, http.StatusCreated, status, "signup failed: body=%s", string(body))
	mustConfirmEmailVerification(t, server, fmt.Sprintf("%s@example.com", username))
}

func mustIssueGuest(t *testing.T, baseURL string) string {
	t.Helper()
	body, status, headers := requestJSON(t, baseURL, "", http.MethodPost, "/auth/guest", map[string]any{})
	assert.Equal(t, http.StatusCreated, status, "issue guest failed: body=%s", string(body))
	token := headers.Get("Authorization")
	require.NotEmpty(t, token)
	return token
}

func mustUpgradeGuest(t *testing.T, server *integrationServer, token, username, email, password string) string {
	t.Helper()
	baseURL := server.URL
	body, status, headers := requestJSON(t, baseURL, token, http.MethodPost, "/auth/guest/upgrade", map[string]any{
		"username": username,
		"email":    email,
		"password": password,
	})
	assert.Equal(t, http.StatusOK, status, "upgrade guest failed: body=%s", string(body))
	newToken := headers.Get("Authorization")
	require.NotEmpty(t, newToken)
	mustConfirmEmailVerification(t, server, email)
	return newToken
}

func mustConfirmEmailVerification(t *testing.T, server *integrationServer, email string) {
	t.Helper()
	token := server.verificationMailer.tokenFor(email)
	require.NotEmpty(t, token)
	body, status, _ := requestJSON(t, server.URL, "", http.MethodPost, "/auth/email-verification/confirm", map[string]any{
		"token": token,
	})
	assert.Equal(t, http.StatusNoContent, status, "email verification confirm failed: body=%s", string(body))
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

func mustCreateBoard(t *testing.T, baseURL, token, name, description string) string {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, token, http.MethodPost, "/boards", map[string]any{
		"name":        name,
		"description": description,
	})
	assert.Equal(t, http.StatusCreated, status, "create board failed: body=%s", string(body))
	var resp map[string]any
	mustUnmarshal(t, body, &resp)
	return resp["uuid"].(string)
}

func mustGetBoards(t *testing.T, baseURL string, expectedBoardUUID string) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, "", http.MethodGet, "/boards", nil)
	assert.Equal(t, http.StatusOK, status, "get boards failed: body=%s", string(body))
	var resp map[string]any
	mustUnmarshal(t, body, &resp)
	boards := resp["boards"].([]any)
	require.NotEmpty(t, boards)
	first := boards[0].(map[string]any)
	assert.Equal(t, expectedBoardUUID, first["uuid"])
}

func mustCreatePost(t *testing.T, baseURL, token string, boardUUID, title, content string) string {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, token, http.MethodPost, fmt.Sprintf("/boards/%s/posts", boardUUID), map[string]any{
		"title":   title,
		"content": content,
	})
	assert.Equal(t, http.StatusCreated, status, "create post failed: body=%s", string(body))
	var resp map[string]any
	mustUnmarshal(t, body, &resp)
	return resp["uuid"].(string)
}

func mustGetPost(t *testing.T, baseURL string, postUUID string) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, "", http.MethodGet, fmt.Sprintf("/posts/%s", postUUID), nil)
	assert.Equal(t, http.StatusOK, status, "get post failed: body=%s", string(body))
	var resp map[string]any
	mustUnmarshal(t, body, &resp)
	post := resp["post"].(map[string]any)
	assert.Equal(t, postUUID, post["uuid"])
	authorUUID, ok := post["author_uuid"].(string)
	require.True(t, ok)
	assert.NotEmpty(t, authorUUID)
	_, hasAuthorID := post["author_id"]
	assert.False(t, hasAuthorID)
}

func mustUpdatePost(t *testing.T, baseURL, token string, postUUID, title, content string) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, token, http.MethodPut, fmt.Sprintf("/posts/%s", postUUID), map[string]any{
		"title":   title,
		"content": content,
	})
	assert.Equal(t, http.StatusNoContent, status, "update post failed: body=%s", string(body))
}

func mustDeletePost(t *testing.T, baseURL, token string, postUUID string) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, token, http.MethodDelete, fmt.Sprintf("/posts/%s", postUUID), nil)
	assert.Equal(t, http.StatusNoContent, status, "delete post failed: body=%s", string(body))
}

func mustCreateComment(t *testing.T, baseURL, token string, postUUID, content string) string {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, token, http.MethodPost, fmt.Sprintf("/posts/%s/comments", postUUID), map[string]any{
		"content": content,
	})
	assert.Equal(t, http.StatusCreated, status, "create comment failed: body=%s", string(body))
	var resp map[string]any
	mustUnmarshal(t, body, &resp)
	return resp["uuid"].(string)
}

func mustCreateReplyComment(t *testing.T, baseURL, token string, postUUID, parentUUID, content string) string {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, token, http.MethodPost, fmt.Sprintf("/posts/%s/comments", postUUID), map[string]any{
		"content":     content,
		"parent_uuid": parentUUID,
	})
	assert.Equal(t, http.StatusCreated, status, "create reply comment failed: body=%s", string(body))
	var resp map[string]any
	mustUnmarshal(t, body, &resp)
	return resp["uuid"].(string)
}

func mustGetComments(t *testing.T, baseURL string, postUUID, expectedCommentUUID string) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, "", http.MethodGet, fmt.Sprintf("/posts/%s/comments", postUUID), nil)
	assert.Equal(t, http.StatusOK, status, "get comments failed: body=%s", string(body))
	var resp map[string]any
	mustUnmarshal(t, body, &resp)
	comments := resp["comments"].([]any)
	require.NotEmpty(t, comments)
	first := comments[0].(map[string]any)
	assert.Equal(t, expectedCommentUUID, first["uuid"])
	authorUUID, ok := first["author_uuid"].(string)
	require.True(t, ok)
	assert.NotEmpty(t, authorUUID)
	_, hasAuthorID := first["author_id"]
	assert.False(t, hasAuthorID)
}

func mustUpdateComment(t *testing.T, baseURL, token string, commentUUID, content string) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, token, http.MethodPut, fmt.Sprintf("/comments/%s", commentUUID), map[string]any{
		"content": content,
	})
	assert.Equal(t, http.StatusNoContent, status, "update comment failed: body=%s", string(body))
}

func mustDeleteComment(t *testing.T, baseURL, token string, commentUUID string) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, token, http.MethodDelete, fmt.Sprintf("/comments/%s", commentUUID), nil)
	assert.Equal(t, http.StatusNoContent, status, "delete comment failed: body=%s", string(body))
}

func mustSetPostReaction(t *testing.T, baseURL, token string, postUUID, reactionType string, expectedStatus int) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, token, http.MethodPut, fmt.Sprintf("/posts/%s/reactions/me", postUUID), map[string]any{
		"reaction_type": reactionType,
	})
	assert.Equal(t, expectedStatus, status, "set post reaction failed: body=%s", string(body))
}

func mustSetCommentReaction(t *testing.T, baseURL, token string, commentUUID, reactionType string, expectedStatus int) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, token, http.MethodPut, fmt.Sprintf("/comments/%s/reactions/me", commentUUID), map[string]any{
		"reaction_type": reactionType,
	})
	assert.Equal(t, expectedStatus, status, "set comment reaction failed: body=%s", string(body))
}

func mustHaveFirstPostReactionType(t *testing.T, baseURL string, postUUID, expectedType string) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, "", http.MethodGet, fmt.Sprintf("/posts/%s/reactions", postUUID), nil)
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

func mustHaveFirstCommentReactionType(t *testing.T, baseURL string, commentUUID, expectedType string) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, "", http.MethodGet, fmt.Sprintf("/comments/%s/reactions", commentUUID), nil)
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

func mustDeletePostReaction(t *testing.T, baseURL, token string, postUUID string) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, token, http.MethodDelete, fmt.Sprintf("/posts/%s/reactions/me", postUUID), nil)
	assert.Equal(t, http.StatusNoContent, status, "delete post reaction failed: body=%s", string(body))
}

func mustDeleteCommentReaction(t *testing.T, baseURL, token string, commentUUID string) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, token, http.MethodDelete, fmt.Sprintf("/comments/%s/reactions/me", commentUUID), nil)
	assert.Equal(t, http.StatusNoContent, status, "delete comment reaction failed: body=%s", string(body))
}

func mustNoComments(t *testing.T, baseURL string, postUUID string) {
	t.Helper()
	require.Eventually(t, func() bool {
		body, status, _ := requestJSON(t, baseURL, "", http.MethodGet, fmt.Sprintf("/posts/%s/comments", postUUID), nil)
		if status != http.StatusOK {
			return false
		}
		var resp map[string]any
		mustUnmarshal(t, body, &resp)
		rawComments, exists := resp["comments"]
		if !exists || rawComments == nil {
			return true
		}
		comments, ok := rawComments.([]any)
		require.True(t, ok, "unexpected comments payload type: %T", rawComments)
		return len(comments) == 0
	}, time.Second, 10*time.Millisecond)
}

func mustHaveDeletedParentAndVisibleReply(t *testing.T, baseURL string, postUUID, parentUUID string) {
	t.Helper()
	body, status, _ := requestJSON(t, baseURL, "", http.MethodGet, fmt.Sprintf("/posts/%s/comments", postUUID), nil)
	assert.Equal(t, http.StatusOK, status, "get comments failed: body=%s", string(body))
	var resp map[string]any
	mustUnmarshal(t, body, &resp)
	comments := resp["comments"].([]any)
	require.Len(t, comments, 2)

	reply := comments[0].(map[string]any)
	assert.Equal(t, "reply", reply["content"])
	assert.Equal(t, parentUUID, reply["parent_uuid"])

	parent := comments[1].(map[string]any)
	assert.Equal(t, parentUUID, parent["uuid"])
	assert.Equal(t, "삭제된 댓글", parent["content"])
}

func mustNoPostReactions(t *testing.T, baseURL string, postUUID string) {
	t.Helper()
	require.Eventually(t, func() bool {
		body, status, _ := requestJSON(t, baseURL, "", http.MethodGet, fmt.Sprintf("/posts/%s/reactions", postUUID), nil)
		if status != http.StatusOK {
			return false
		}
		var resp []map[string]any
		mustUnmarshal(t, body, &resp)
		return len(resp) == 0
	}, time.Second, 10*time.Millisecond)
}

func mustNoCommentReactions(t *testing.T, baseURL string, commentUUID string) {
	t.Helper()
	require.Eventually(t, func() bool {
		body, status, _ := requestJSON(t, baseURL, "", http.MethodGet, fmt.Sprintf("/comments/%s/reactions", commentUUID), nil)
		if status != http.StatusOK {
			return false
		}
		var resp []map[string]any
		mustUnmarshal(t, body, &resp)
		return len(resp) == 0
	}, time.Second, 10*time.Millisecond)
}

func mustPostNotAccessible(t *testing.T, baseURL string, postUUID string) {
	t.Helper()
	require.Eventually(t, func() bool {
		_, status, _ := requestJSON(t, baseURL, "", http.MethodGet, fmt.Sprintf("/posts/%s", postUUID), nil)
		return status == http.StatusNotFound
	}, time.Second, 10*time.Millisecond)
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
