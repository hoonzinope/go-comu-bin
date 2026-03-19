package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/application/service"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/hoonzinope/go-comu-bin/internal/infrastructure/auth"
	cacheInMemory "github.com/hoonzinope/go-comu-bin/internal/infrastructure/cache/inmemory"
	rateLimitInMemory "github.com/hoonzinope/go-comu-bin/internal/infrastructure/ratelimit/inmemory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const apiV1Prefix = "/api/v1"

type fakeUserUseCase struct {
	signUp            func(ctx context.Context, username, password string) (string, error)
	issueGuestAccount func(ctx context.Context) (int64, error)
	upgradeGuest      func(ctx context.Context, userID int64, username, email, password string) error
	deleteMe          func(ctx context.Context, userID int64, password string) error
	getUserSuspension func(ctx context.Context, adminID int64, targetUserUUID string) (*model.UserSuspension, error)
	suspendUser       func(ctx context.Context, adminID int64, targetUserUUID, reason string, duration model.SuspensionDuration) error
	unsuspendUser     func(ctx context.Context, adminID int64, targetUserUUID string) error
	verifyCredential  func(ctx context.Context, username, password string) (int64, error)
	ensureAdmin       func(ctx context.Context, userID int64) error
}

func (f *fakeUserUseCase) SignUp(ctx context.Context, username, password string) (string, error) {
	if f.signUp != nil {
		return f.signUp(ctx, username, password)
	}
	return "ok", nil
}

func (f *fakeUserUseCase) IssueGuestAccount(ctx context.Context) (int64, error) {
	if f.issueGuestAccount != nil {
		return f.issueGuestAccount(ctx)
	}
	return 42, nil
}

func (f *fakeUserUseCase) UpgradeGuest(ctx context.Context, userID int64, username, email, password string) error {
	if f.upgradeGuest != nil {
		return f.upgradeGuest(ctx, userID, username, email, password)
	}
	return nil
}

func (f *fakeUserUseCase) DeleteMe(ctx context.Context, userID int64, password string) error {
	if f.deleteMe != nil {
		return f.deleteMe(ctx, userID, password)
	}
	return nil
}

func (f *fakeUserUseCase) GetUserSuspension(ctx context.Context, adminID int64, targetUserUUID string) (*model.UserSuspension, error) {
	if f.getUserSuspension != nil {
		return f.getUserSuspension(ctx, adminID, targetUserUUID)
	}
	return &model.UserSuspension{}, nil
}

func (f *fakeUserUseCase) SuspendUser(ctx context.Context, adminID int64, targetUserUUID, reason string, duration model.SuspensionDuration) error {
	if f.suspendUser != nil {
		return f.suspendUser(ctx, adminID, targetUserUUID, reason, duration)
	}
	return nil
}

func (f *fakeUserUseCase) UnsuspendUser(ctx context.Context, adminID int64, targetUserUUID string) error {
	if f.unsuspendUser != nil {
		return f.unsuspendUser(ctx, adminID, targetUserUUID)
	}
	return nil
}

func (f *fakeUserUseCase) VerifyCredentials(ctx context.Context, username, password string) (int64, error) {
	if f.verifyCredential != nil {
		return f.verifyCredential(ctx, username, password)
	}
	return 1, nil
}

func (f *fakeUserUseCase) EnsureAdmin(ctx context.Context, userID int64) error {
	if f.ensureAdmin != nil {
		return f.ensureAdmin(ctx, userID)
	}
	user, err := f.SelectUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil {
		return customerror.ErrUserNotFound
	}
	if !user.IsAdmin() {
		return customerror.ErrForbidden
	}
	return nil
}

func (f *fakeUserUseCase) Save(context.Context, *entity.User) (int64, error) {
	return 1, nil
}

func (f *fakeUserUseCase) SelectUserByUsername(_ context.Context, username string) (*entity.User, error) {
	return &entity.User{ID: 1, Name: username, Status: entity.UserStatusActive}, nil
}

func (f *fakeUserUseCase) SelectUserByUUID(_ context.Context, userUUID string) (*entity.User, error) {
	return &entity.User{ID: 1, UUID: userUUID, Status: entity.UserStatusActive}, nil
}

func (f *fakeUserUseCase) SelectUserByID(_ context.Context, id int64) (*entity.User, error) {
	if id == 1 {
		return &entity.User{ID: id, Name: "admin", Role: "admin", Status: entity.UserStatusActive}, nil
	}
	return &entity.User{ID: id, Name: "user", Status: entity.UserStatusActive}, nil
}

func (f *fakeUserUseCase) SelectUserByIDIncludingDeleted(_ context.Context, id int64) (*entity.User, error) {
	return &entity.User{ID: id, Name: "user", Status: entity.UserStatusActive}, nil
}

func (f *fakeUserUseCase) SelectUsersByIDsIncludingDeleted(_ context.Context, ids []int64) (map[int64]*entity.User, error) {
	out := make(map[int64]*entity.User, len(ids))
	for _, id := range ids {
		out[id] = &entity.User{ID: id, Name: "user", Status: entity.UserStatusActive}
	}
	return out, nil
}

func (f *fakeUserUseCase) SelectGuestCleanupCandidates(_ context.Context, now time.Time, pendingGrace, activeUnusedGrace time.Duration, limit int) ([]*entity.User, error) {
	return []*entity.User{}, nil
}

func (f *fakeUserUseCase) Update(context.Context, *entity.User) error {
	return nil
}

func (f *fakeUserUseCase) Delete(context.Context, int64) error {
	return nil
}

type fakeAccountUseCase struct {
	deleteMyAccount func(ctx context.Context, userID int64, password string) error
}

func (f *fakeAccountUseCase) DeleteMyAccount(ctx context.Context, userID int64, password string) error {
	if f.deleteMyAccount != nil {
		return f.deleteMyAccount(ctx, userID, password)
	}
	return nil
}

func TestSwaggerContracts_ResponseSchemasMatchHandlers(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "swagger", "swagger.json"))
	require.NoError(t, err)

	var spec map[string]any
	require.NoError(t, json.Unmarshal(data, &spec))

	paths, ok := spec["paths"].(map[string]any)
	require.True(t, ok)

	assert.Equal(
		t,
		"#/definitions/delivery.uuidResponse",
		swaggerResponseSchemaRef(t, paths, "/boards", "post", "201"),
	)
	assert.Equal(
		t,
		"#/definitions/delivery.attachmentUploadResponse",
		swaggerResponseSchemaRef(t, paths, "/posts/{postUUID}/attachments/upload", "post", "201"),
	)
}

func swaggerResponseSchemaRef(t *testing.T, paths map[string]any, path, method, status string) string {
	t.Helper()

	pathItem, ok := paths[path].(map[string]any)
	require.True(t, ok, "missing swagger path: %s", path)

	operation, ok := pathItem[method].(map[string]any)
	require.True(t, ok, "missing swagger method %s %s", method, path)

	responses, ok := operation["responses"].(map[string]any)
	require.True(t, ok, "missing swagger responses for %s %s", method, path)

	response, ok := responses[status].(map[string]any)
	require.True(t, ok, "missing swagger response %s for %s %s", status, method, path)

	schema, ok := response["schema"].(map[string]any)
	require.True(t, ok, "missing swagger schema %s for %s %s", status, method, path)

	ref, ok := schema["$ref"].(string)
	require.True(t, ok, "missing swagger schema ref %s for %s %s", status, method, path)

	return ref
}

type spyLogger struct {
	warns  int
	errors int
}

func (l *spyLogger) Logger() *slog.Logger {
	return slog.New(&spyHandler{logger: l})
}

type spyHandler struct {
	logger *spyLogger
}

func (h *spyHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *spyHandler) Handle(_ context.Context, record slog.Record) error {
	if record.Level >= slog.LevelError {
		h.logger.errors++
	} else if record.Level >= slog.LevelWarn {
		h.logger.warns++
	}
	return nil
}

func (h *spyHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h *spyHandler) WithGroup(string) slog.Handler { return h }

type fakeBoardUseCase struct {
	getBoards          func(ctx context.Context, limit int, cursor string) (*model.BoardList, error)
	createBoard        func(ctx context.Context, userID int64, name, description string) (string, error)
	updateBoard        func(ctx context.Context, boardUUID string, userID int64, name, description string) error
	deleteBoard        func(ctx context.Context, boardUUID string, userID int64) error
	setBoardVisibility func(ctx context.Context, boardUUID string, userID int64, hidden bool) error
}

func (f *fakeBoardUseCase) GetBoards(ctx context.Context, limit int, cursor string) (*model.BoardList, error) {
	if f.getBoards != nil {
		return f.getBoards(ctx, limit, cursor)
	}
	return &model.BoardList{}, nil
}

func (f *fakeBoardUseCase) CreateBoard(ctx context.Context, userID int64, name, description string) (string, error) {
	if f.createBoard != nil {
		return f.createBoard(ctx, userID, name, description)
	}
	return "board-uuid-1", nil
}

func (f *fakeBoardUseCase) UpdateBoard(ctx context.Context, boardUUID string, userID int64, name, description string) error {
	if f.updateBoard != nil {
		return f.updateBoard(ctx, boardUUID, userID, name, description)
	}
	return nil
}

func (f *fakeBoardUseCase) DeleteBoard(ctx context.Context, boardUUID string, userID int64) error {
	if f.deleteBoard != nil {
		return f.deleteBoard(ctx, boardUUID, userID)
	}
	return nil
}

func (f *fakeBoardUseCase) SetBoardVisibility(ctx context.Context, boardUUID string, userID int64, hidden bool) error {
	if f.setBoardVisibility != nil {
		return f.setBoardVisibility(ctx, boardUUID, userID, hidden)
	}
	return nil
}

type fakeReportUseCase struct {
	createReport  func(ctx context.Context, reporterUserID int64, targetType model.ReportTargetType, targetUUID string, reasonCode model.ReportReasonCode, reasonDetail string) (int64, error)
	getReports    func(ctx context.Context, adminID int64, status *model.ReportStatus, limit int, lastID int64) (*model.ReportList, error)
	resolveReport func(ctx context.Context, adminID, reportID int64, status model.ReportStatus, resolutionNote string) error
}

func (f *fakeReportUseCase) CreateReport(ctx context.Context, reporterUserID int64, targetType model.ReportTargetType, targetUUID string, reasonCode model.ReportReasonCode, reasonDetail string) (int64, error) {
	if f.createReport != nil {
		return f.createReport(ctx, reporterUserID, targetType, targetUUID, reasonCode, reasonDetail)
	}
	return 1, nil
}

func (f *fakeReportUseCase) GetReports(ctx context.Context, adminID int64, status *model.ReportStatus, limit int, lastID int64) (*model.ReportList, error) {
	if f.getReports != nil {
		return f.getReports(ctx, adminID, status, limit, lastID)
	}
	return &model.ReportList{}, nil
}

func (f *fakeReportUseCase) ResolveReport(ctx context.Context, adminID, reportID int64, status model.ReportStatus, resolutionNote string) error {
	if f.resolveReport != nil {
		return f.resolveReport(ctx, adminID, reportID, status, resolutionNote)
	}
	return nil
}

type fakeOutboxAdminUseCase struct {
	getDeadMessages    func(ctx context.Context, adminID int64, limit int, lastID string) (*model.OutboxDeadMessageList, error)
	requeueDeadMessage func(ctx context.Context, adminID int64, messageID string) error
	discardDeadMessage func(ctx context.Context, adminID int64, messageID string) error
}

func (f *fakeOutboxAdminUseCase) GetDeadMessages(ctx context.Context, adminID int64, limit int, lastID string) (*model.OutboxDeadMessageList, error) {
	if f.getDeadMessages != nil {
		return f.getDeadMessages(ctx, adminID, limit, lastID)
	}
	return &model.OutboxDeadMessageList{}, nil
}

func (f *fakeOutboxAdminUseCase) RequeueDeadMessage(ctx context.Context, adminID int64, messageID string) error {
	if f.requeueDeadMessage != nil {
		return f.requeueDeadMessage(ctx, adminID, messageID)
	}
	return nil
}

func (f *fakeOutboxAdminUseCase) DiscardDeadMessage(ctx context.Context, adminID int64, messageID string) error {
	if f.discardDeadMessage != nil {
		return f.discardDeadMessage(ctx, adminID, messageID)
	}
	return nil
}

type fakePostUseCase struct {
	createPost      func(ctx context.Context, title, content string, tags []string, authorID int64, boardUUID string) (string, error)
	createDraftPost func(ctx context.Context, title, content string, tags []string, authorID int64, boardUUID string) (string, error)
	getPostsList    func(ctx context.Context, boardUUID string, limit int, cursor string) (*model.PostList, error)
	getPostsByTag   func(ctx context.Context, tagName string, limit int, cursor string) (*model.PostList, error)
	getPostDetail   func(ctx context.Context, postUUID string) (*model.PostDetail, error)
	publishPost     func(ctx context.Context, postUUID string, authorID int64) error
	updatePost      func(ctx context.Context, postUUID string, authorID int64, title, content string, tags []string) error
	deletePost      func(ctx context.Context, postUUID string, authorID int64) error
}

func (f *fakePostUseCase) CreatePost(ctx context.Context, title, content string, tags []string, authorID int64, boardUUID string) (string, error) {
	if f.createPost != nil {
		return f.createPost(ctx, title, content, tags, authorID, boardUUID)
	}
	return "post-uuid-1", nil
}

func (f *fakePostUseCase) CreateDraftPost(ctx context.Context, title, content string, tags []string, authorID int64, boardUUID string) (string, error) {
	if f.createDraftPost != nil {
		return f.createDraftPost(ctx, title, content, tags, authorID, boardUUID)
	}
	return "post-uuid-1", nil
}

func (f *fakePostUseCase) GetPostsList(ctx context.Context, boardUUID string, limit int, cursor string) (*model.PostList, error) {
	if f.getPostsList != nil {
		return f.getPostsList(ctx, boardUUID, limit, cursor)
	}
	return &model.PostList{}, nil
}

func (f *fakePostUseCase) GetPostsByTag(ctx context.Context, tagName string, limit int, cursor string) (*model.PostList, error) {
	if f.getPostsByTag != nil {
		return f.getPostsByTag(ctx, tagName, limit, cursor)
	}
	return &model.PostList{}, nil
}

func (f *fakePostUseCase) GetPostDetail(ctx context.Context, postUUID string) (*model.PostDetail, error) {
	if f.getPostDetail != nil {
		return f.getPostDetail(ctx, postUUID)
	}
	return &model.PostDetail{}, nil
}

func (f *fakePostUseCase) PublishPost(ctx context.Context, postUUID string, authorID int64) error {
	if f.publishPost != nil {
		return f.publishPost(ctx, postUUID, authorID)
	}
	return nil
}

func (f *fakePostUseCase) UpdatePost(ctx context.Context, postUUID string, authorID int64, title, content string, tags []string) error {
	if f.updatePost != nil {
		return f.updatePost(ctx, postUUID, authorID, title, content, tags)
	}
	return nil
}

func (f *fakePostUseCase) DeletePost(ctx context.Context, postUUID string, authorID int64) error {
	if f.deletePost != nil {
		return f.deletePost(ctx, postUUID, authorID)
	}
	return nil
}

type fakeCommentUseCase struct {
	createComment     func(ctx context.Context, content string, authorID int64, postUUID string, parentUUID *string) (string, error)
	getCommentsByPost func(ctx context.Context, postUUID string, limit int, cursor string) (*model.CommentList, error)
	updateComment     func(ctx context.Context, commentUUID string, authorID int64, content string) error
	deleteComment     func(ctx context.Context, commentUUID string, authorID int64) error
}

func (f *fakeCommentUseCase) CreateComment(ctx context.Context, content string, authorID int64, postUUID string, parentUUID *string) (string, error) {
	if f.createComment != nil {
		return f.createComment(ctx, content, authorID, postUUID, parentUUID)
	}
	return "comment-uuid-1", nil
}

func (f *fakeCommentUseCase) GetCommentsByPost(ctx context.Context, postUUID string, limit int, cursor string) (*model.CommentList, error) {
	if f.getCommentsByPost != nil {
		return f.getCommentsByPost(ctx, postUUID, limit, cursor)
	}
	return &model.CommentList{}, nil
}

func (f *fakeCommentUseCase) UpdateComment(ctx context.Context, commentUUID string, authorID int64, content string) error {
	if f.updateComment != nil {
		return f.updateComment(ctx, commentUUID, authorID, content)
	}
	return nil
}

func (f *fakeCommentUseCase) DeleteComment(ctx context.Context, commentUUID string, authorID int64) error {
	if f.deleteComment != nil {
		return f.deleteComment(ctx, commentUUID, authorID)
	}
	return nil
}

type fakeReactionUseCase struct {
	setReaction          func(ctx context.Context, userID int64, targetUUID string, targetType model.ReactionTargetType, reactionType model.ReactionType) (bool, error)
	deleteReaction       func(ctx context.Context, userID int64, targetUUID string, targetType model.ReactionTargetType) error
	getReactionsByTarget func(ctx context.Context, targetUUID string, targetType model.ReactionTargetType) ([]model.Reaction, error)
}

type fakeAttachmentUseCase struct {
	createPostAttachment         func(ctx context.Context, postUUID string, userID int64, fileName, contentType string, sizeBytes int64, storageKey string) (string, error)
	getPostAttachments           func(ctx context.Context, postUUID string) ([]model.Attachment, error)
	getPostAttachmentFile        func(ctx context.Context, postUUID, attachmentUUID string) (*model.AttachmentFile, error)
	getPostAttachmentPreviewFile func(ctx context.Context, postUUID, attachmentUUID string, userID int64) (*model.AttachmentFile, error)
	deletePostAttachment         func(ctx context.Context, postUUID, attachmentUUID string, userID int64) error
	uploadPostAttachment         func(ctx context.Context, postUUID string, userID int64, fileName, contentType string, content io.Reader) (*model.AttachmentUpload, error)
}

func (f *fakeAttachmentUseCase) CreatePostAttachment(ctx context.Context, postUUID string, userID int64, fileName, contentType string, sizeBytes int64, storageKey string) (string, error) {
	if f.createPostAttachment != nil {
		return f.createPostAttachment(ctx, postUUID, userID, fileName, contentType, sizeBytes, storageKey)
	}
	return "attachment-uuid-1", nil
}

func (f *fakeAttachmentUseCase) GetPostAttachments(ctx context.Context, postUUID string) ([]model.Attachment, error) {
	if f.getPostAttachments != nil {
		return f.getPostAttachments(ctx, postUUID)
	}
	return []model.Attachment{}, nil
}

func (f *fakeAttachmentUseCase) GetPostAttachmentFile(ctx context.Context, postUUID, attachmentUUID string) (*model.AttachmentFile, error) {
	if f.getPostAttachmentFile != nil {
		return f.getPostAttachmentFile(ctx, postUUID, attachmentUUID)
	}
	return nil, customerror.ErrAttachmentNotFound
}

func (f *fakeAttachmentUseCase) GetPostAttachmentPreviewFile(ctx context.Context, postUUID, attachmentUUID string, userID int64) (*model.AttachmentFile, error) {
	if f.getPostAttachmentPreviewFile != nil {
		return f.getPostAttachmentPreviewFile(ctx, postUUID, attachmentUUID, userID)
	}
	return nil, customerror.ErrAttachmentNotFound
}

func (f *fakeAttachmentUseCase) DeletePostAttachment(ctx context.Context, postUUID, attachmentUUID string, userID int64) error {
	if f.deletePostAttachment != nil {
		return f.deletePostAttachment(ctx, postUUID, attachmentUUID, userID)
	}
	return nil
}

func (f *fakeAttachmentUseCase) UploadPostAttachment(ctx context.Context, postUUID string, userID int64, fileName, contentType string, content io.Reader) (*model.AttachmentUpload, error) {
	if f.uploadPostAttachment != nil {
		return f.uploadPostAttachment(ctx, postUUID, userID, fileName, contentType, content)
	}
	return &model.AttachmentUpload{UUID: "attachment-uuid-1", EmbedMarkdown: "![a.png](attachment://attachment-uuid-1)"}, nil
}

var testSessionRepository port.SessionRepository

type authUserPort interface {
	port.UserUseCase
	port.AdminAuthorizer
	port.CredentialVerifier
	port.UserRepository
}

func (f *fakeReactionUseCase) SetReaction(ctx context.Context, userID int64, targetUUID string, targetType model.ReactionTargetType, reactionType model.ReactionType) (bool, error) {
	if f.setReaction != nil {
		return f.setReaction(ctx, userID, targetUUID, targetType, reactionType)
	}
	return false, nil
}

func (f *fakeReactionUseCase) DeleteReaction(ctx context.Context, userID int64, targetUUID string, targetType model.ReactionTargetType) error {
	if f.deleteReaction != nil {
		return f.deleteReaction(ctx, userID, targetUUID, targetType)
	}
	return nil
}

func (f *fakeReactionUseCase) GetReactionsByTarget(ctx context.Context, targetUUID string, targetType model.ReactionTargetType) ([]model.Reaction, error) {
	if f.getReactionsByTarget != nil {
		return f.getReactionsByTarget(ctx, targetUUID, targetType)
	}
	return []model.Reaction{}, nil
}

func newTestHandler(
	user authUserPort,
	account port.AccountUseCase,
	board port.BoardUseCase,
	post port.PostUseCase,
	comment port.CommentUseCase,
	reaction port.ReactionUseCase,
	attachment port.AttachmentUseCase,
) http.Handler {
	tokenProvider := auth.NewJwtTokenProvider("test-secret")
	testSessionRepository = auth.NewCacheSessionRepository(cacheInMemory.NewInMemoryCache())
	sessionUseCase := service.NewSessionService(user, user, user, tokenProvider, testSessionRepository)
	reportUseCase := &fakeReportUseCase{}
	outboxAdminUseCase := &fakeOutboxAdminUseCase{}
	return NewHTTPServer(":0", HTTPDependencies{
		SessionUseCase:     sessionUseCase,
		AdminAuthorizer:    user,
		UserUseCase:        user,
		AccountUseCase:     account,
		BoardUseCase:       board,
		PostUseCase:        post,
		CommentUseCase:     comment,
		ReactionUseCase:    reaction,
		AttachmentUseCase:  attachment,
		ReportUseCase:      reportUseCase,
		OutboxAdminUseCase: outboxAdminUseCase,
		MaxJSONBodyBytes:   defaultMaxJSONBodyBytes,
	}).Handler
}

func newTestHandlerWithJSONLimit(
	user authUserPort,
	account port.AccountUseCase,
	board port.BoardUseCase,
	post port.PostUseCase,
	comment port.CommentUseCase,
	reaction port.ReactionUseCase,
	attachment port.AttachmentUseCase,
	maxJSONBodyBytes int64,
) http.Handler {
	tokenProvider := auth.NewJwtTokenProvider("test-secret")
	testSessionRepository = auth.NewCacheSessionRepository(cacheInMemory.NewInMemoryCache())
	sessionUseCase := service.NewSessionService(user, user, user, tokenProvider, testSessionRepository)
	reportUseCase := &fakeReportUseCase{}
	outboxAdminUseCase := &fakeOutboxAdminUseCase{}
	return NewHTTPServer(":0", HTTPDependencies{
		SessionUseCase:     sessionUseCase,
		AdminAuthorizer:    user,
		UserUseCase:        user,
		AccountUseCase:     account,
		BoardUseCase:       board,
		PostUseCase:        post,
		CommentUseCase:     comment,
		ReactionUseCase:    reaction,
		AttachmentUseCase:  attachment,
		ReportUseCase:      reportUseCase,
		OutboxAdminUseCase: outboxAdminUseCase,
		MaxJSONBodyBytes:   maxJSONBodyBytes,
	}).Handler
}

func newTestHandlerWithDefaultPageLimit(
	user authUserPort,
	account port.AccountUseCase,
	board port.BoardUseCase,
	post port.PostUseCase,
	comment port.CommentUseCase,
	reaction port.ReactionUseCase,
	attachment port.AttachmentUseCase,
	defaultPageLimit int,
) http.Handler {
	tokenProvider := auth.NewJwtTokenProvider("test-secret")
	testSessionRepository = auth.NewCacheSessionRepository(cacheInMemory.NewInMemoryCache())
	sessionUseCase := service.NewSessionService(user, user, user, tokenProvider, testSessionRepository)
	reportUseCase := &fakeReportUseCase{}
	outboxAdminUseCase := &fakeOutboxAdminUseCase{}
	return NewHTTPServer(":0", HTTPDependencies{
		SessionUseCase:     sessionUseCase,
		AdminAuthorizer:    user,
		UserUseCase:        user,
		AccountUseCase:     account,
		BoardUseCase:       board,
		PostUseCase:        post,
		CommentUseCase:     comment,
		ReactionUseCase:    reaction,
		AttachmentUseCase:  attachment,
		ReportUseCase:      reportUseCase,
		OutboxAdminUseCase: outboxAdminUseCase,
		MaxJSONBodyBytes:   defaultMaxJSONBodyBytes,
		DefaultPageLimit:   defaultPageLimit,
	}).Handler
}

func newTestHandlerWithRateLimit(
	user authUserPort,
	account port.AccountUseCase,
	board port.BoardUseCase,
	post port.PostUseCase,
	comment port.CommentUseCase,
	reaction port.ReactionUseCase,
	attachment port.AttachmentUseCase,
	enabled bool,
	windowSeconds int,
	readRequests int,
	writeRequests int,
) http.Handler {
	tokenProvider := auth.NewJwtTokenProvider("test-secret")
	testSessionRepository = auth.NewCacheSessionRepository(cacheInMemory.NewInMemoryCache())
	sessionUseCase := service.NewSessionService(user, user, user, tokenProvider, testSessionRepository)
	reportUseCase := &fakeReportUseCase{}
	outboxAdminUseCase := &fakeOutboxAdminUseCase{}
	rateLimiter := rateLimitInMemory.NewInMemoryRateLimiter()
	return NewHTTPServer(":0", HTTPDependencies{
		SessionUseCase:        sessionUseCase,
		AdminAuthorizer:       user,
		UserUseCase:           user,
		AccountUseCase:        account,
		BoardUseCase:          board,
		PostUseCase:           post,
		CommentUseCase:        comment,
		ReactionUseCase:       reaction,
		AttachmentUseCase:     attachment,
		ReportUseCase:         reportUseCase,
		OutboxAdminUseCase:    outboxAdminUseCase,
		RateLimiter:           rateLimiter,
		MaxJSONBodyBytes:      defaultMaxJSONBodyBytes,
		RateLimitEnabled:      enabled,
		RateLimitWindowSecond: windowSeconds,
		RateLimitReadRequest:  readRequests,
		RateLimitWriteRequest: writeRequests,
	}).Handler
}

func newTestHandlerWithAdminUseCases(
	user authUserPort,
	account port.AccountUseCase,
	board port.BoardUseCase,
	post port.PostUseCase,
	comment port.CommentUseCase,
	reaction port.ReactionUseCase,
	attachment port.AttachmentUseCase,
	report port.ReportUseCase,
	outboxAdmin port.OutboxAdminUseCase,
) http.Handler {
	tokenProvider := auth.NewJwtTokenProvider("test-secret")
	testSessionRepository = auth.NewCacheSessionRepository(cacheInMemory.NewInMemoryCache())
	sessionUseCase := service.NewSessionService(user, user, user, tokenProvider, testSessionRepository)
	return NewHTTPServer(":0", HTTPDependencies{
		SessionUseCase:     sessionUseCase,
		AdminAuthorizer:    user,
		UserUseCase:        user,
		AccountUseCase:     account,
		BoardUseCase:       board,
		PostUseCase:        post,
		CommentUseCase:     comment,
		ReactionUseCase:    reaction,
		AttachmentUseCase:  attachment,
		ReportUseCase:      report,
		OutboxAdminUseCase: outboxAdmin,
		MaxJSONBodyBytes:   defaultMaxJSONBodyBytes,
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

type authRequestOption func(*http.Request)

func withAuthToken(token string) authRequestOption {
	return func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

func doJSONRequestWithAuth(t *testing.T, handler http.Handler, method, path string, body any, userID int64, opts ...authRequestOption) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}

	req := httptest.NewRequest(method, apiV1Prefix+path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if len(opts) == 0 {
		tokenProvider := auth.NewJwtTokenProvider("test-secret")
		token, err := tokenProvider.IdToToken(userID)
		require.NoError(t, err)
		require.NotNil(t, testSessionRepository)
		require.NoError(t, testSessionRepository.Save(context.Background(), userID, token, tokenProvider.TTLSeconds()))
		req.Header.Set("Authorization", "Bearer "+token)
	} else {
		for _, opt := range opts {
			opt(req)
		}
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func TestHandleUserSuspend_Success(t *testing.T) {
	expectedTargetUserUUID := "550e8400-e29b-41d4-a716-446655440007"
	handler := newTestHandler(
		&fakeUserUseCase{
			suspendUser: func(ctx context.Context, adminID int64, targetUserUUID, reason string, duration model.SuspensionDuration) error {
				assert.Equal(t, int64(1), adminID)
				assert.Equal(t, expectedTargetUserUUID, targetUserUUID)
				assert.Equal(t, "spam", reason)
				assert.Equal(t, model.SuspensionDuration7Days, duration)
				return nil
			},
		},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{},
	)

	rr := doJSONRequestWithAuth(t, handler, http.MethodPut, "/users/"+expectedTargetUserUUID+"/suspension", map[string]any{
		"reason":   "spam",
		"duration": "7d",
	}, 1)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestHandleReportCreate_Success(t *testing.T) {
	targetUUID := "550e8400-e29b-41d4-a716-446655440003"
	handler := newTestHandlerWithAdminUseCases(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{},
		&fakeReportUseCase{
			createReport: func(ctx context.Context, reporterUserID int64, targetType model.ReportTargetType, targetUUIDArg string, reasonCode model.ReportReasonCode, reasonDetail string) (int64, error) {
				assert.Equal(t, int64(1), reporterUserID)
				assert.Equal(t, model.ReportTargetPost, targetType)
				assert.Equal(t, targetUUID, targetUUIDArg)
				assert.Equal(t, model.ReportReasonSpam, reasonCode)
				assert.Equal(t, "detail", reasonDetail)
				return 77, nil
			},
		},
		&fakeOutboxAdminUseCase{},
	)

	rr := doJSONRequestWithAuth(t, handler, http.MethodPost, "/reports", map[string]any{
		"target_type":   "post",
		"target_uuid":   targetUUID,
		"reason_code":   "spam",
		"reason_detail": "detail",
	}, 1)

	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.Contains(t, rr.Body.String(), `"id":77`)
}

func TestHandleAdminReportsGet_OmitsInternalForeignKeys(t *testing.T) {
	resolvedByUUID := "550e8400-e29b-41d4-a716-446655440099"
	handler := newTestHandlerWithAdminUseCases(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{},
		&fakeReportUseCase{
			getReports: func(ctx context.Context, adminID int64, status *model.ReportStatus, limit int, lastID int64) (*model.ReportList, error) {
				return &model.ReportList{
					Reports: []model.Report{{
						ID:             7,
						TargetType:     "post",
						TargetUUID:     "550e8400-e29b-41d4-a716-446655440101",
						ReporterUUID:   "550e8400-e29b-41d4-a716-446655440011",
						ReasonCode:     "spam",
						Status:         "pending",
						ResolvedByUUID: &resolvedByUUID,
						ResolvedAt:     nil,
					}},
					Limit:   limit,
					LastID:  lastID,
					HasMore: false,
				}, nil
			},
		},
		&fakeOutboxAdminUseCase{},
	)

	rr := doJSONRequestWithAuth(t, handler, http.MethodGet, "/admin/reports?limit=10", nil, 1)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"reporter_uuid":"550e8400-e29b-41d4-a716-446655440011"`)
	assert.Contains(t, rr.Body.String(), `"resolved_by_uuid":"550e8400-e29b-41d4-a716-446655440099"`)
	assert.NotContains(t, rr.Body.String(), `"target_id"`)
	assert.NotContains(t, rr.Body.String(), `"reporter_user_id"`)
	assert.NotContains(t, rr.Body.String(), `"resolved_by":`)
}

func TestHandleReportCreate_BadRequestForMalformedTargetUUID(t *testing.T) {
	handler := newTestHandlerWithAdminUseCases(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{},
		&fakeReportUseCase{
			createReport: func(ctx context.Context, reporterUserID int64, targetType model.ReportTargetType, targetUUIDArg string, reasonCode model.ReportReasonCode, reasonDetail string) (int64, error) {
				t.Fatal("service should not be called for malformed target_uuid")
				return 0, nil
			},
		},
		&fakeOutboxAdminUseCase{},
	)

	rr := doJSONRequestWithAuth(t, handler, http.MethodPost, "/reports", map[string]any{
		"target_type": "post",
		"target_uuid": "not-a-uuid",
		"reason_code": "spam",
	}, 1)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.JSONEq(t, `{"error":"invalid target_uuid"}`, rr.Body.String())
}

func TestHandleAdminBoardVisibilityPut_Success(t *testing.T) {
	boardUUID := "550e8400-e29b-41d4-a716-446655440008"
	handler := newTestHandlerWithAdminUseCases(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{
			setBoardVisibility: func(ctx context.Context, boardUUIDArg string, userID int64, hidden bool) error {
				assert.Equal(t, boardUUID, boardUUIDArg)
				assert.Equal(t, int64(1), userID)
				assert.True(t, hidden)
				return nil
			},
		},
		&fakePostUseCase{},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{},
		&fakeReportUseCase{},
		&fakeOutboxAdminUseCase{},
	)

	rr := doJSONRequestWithAuth(t, handler, http.MethodPut, "/admin/boards/"+boardUUID+"/visibility", map[string]any{"hidden": true}, 1)
	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestHandleAdminBoardVisibilityPut_ForbiddenForNonAdminAtHTTPBoundary(t *testing.T) {
	boardUUID := "550e8400-e29b-41d4-a716-446655440008"
	handler := newTestHandlerWithAdminUseCases(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{
			setBoardVisibility: func(ctx context.Context, boardUUIDArg string, userID int64, hidden bool) error {
				t.Fatal("service should not be called for non-admin at HTTP boundary")
				return nil
			},
		},
		&fakePostUseCase{},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{},
		&fakeReportUseCase{},
		&fakeOutboxAdminUseCase{},
	)

	rr := doJSONRequestWithAuth(t, handler, http.MethodPut, "/admin/boards/"+boardUUID+"/visibility", map[string]any{"hidden": true}, 2)
	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.JSONEq(t, `{"error":"forbidden"}`, rr.Body.String())
}

func TestHandleAdminDeadOutboxRequeue_Success(t *testing.T) {
	handler := newTestHandlerWithAdminUseCases(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{},
		&fakeReportUseCase{},
		&fakeOutboxAdminUseCase{
			requeueDeadMessage: func(ctx context.Context, adminID int64, messageID string) error {
				assert.Equal(t, int64(1), adminID)
				assert.Equal(t, "dead-1", messageID)
				return nil
			},
		},
	)

	rr := doJSONRequestWithAuth(t, handler, http.MethodPost, "/admin/outbox/dead/dead-1/requeue", nil, 1)
	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestHandleUserSuspensionGet_Success(t *testing.T) {
	until := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	expectedTargetUserUUID := "550e8400-e29b-41d4-a716-446655440007"
	handler := newTestHandler(
		&fakeUserUseCase{
			getUserSuspension: func(ctx context.Context, adminID int64, targetUserUUID string) (*model.UserSuspension, error) {
				assert.Equal(t, int64(1), adminID)
				assert.Equal(t, expectedTargetUserUUID, targetUserUUID)
				return &model.UserSuspension{
					UserUUID:       expectedTargetUserUUID,
					Status:         entity.UserStatusSuspended,
					Reason:         "spam",
					SuspendedUntil: &until,
				}, nil
			},
		},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{},
	)

	rr := doJSONRequestWithAuth(t, handler, http.MethodGet, "/users/"+expectedTargetUserUUID+"/suspension", nil, 1)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.JSONEq(t, `{
			"user_uuid": "550e8400-e29b-41d4-a716-446655440007",
			"status": "suspended",
			"reason": "spam",
			"suspended_until": "2026-03-15T10:00:00Z"
		}`, rr.Body.String())
}

func TestHandleCreateDraftPost_Success(t *testing.T) {
	boardUUID := "550e8400-e29b-41d4-a716-446655440003"
	handler := newTestHandler(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{
			createDraftPost: func(ctx context.Context, title, content string, tags []string, authorID int64, boardUUIDArg string) (string, error) {
				assert.Equal(t, "draft", title)
				assert.Equal(t, "content", content)
				assert.Nil(t, tags)
				assert.Equal(t, int64(1), authorID)
				assert.Equal(t, boardUUID, boardUUIDArg)
				return "550e8400-e29b-41d4-a716-446655440009", nil
			},
		},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{},
	)

	rr := doJSONRequestWithAuth(t, handler, http.MethodPost, "/boards/"+boardUUID+"/posts/drafts", map[string]string{
		"title":   "draft",
		"content": "content",
	}, 1)

	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.JSONEq(t, `{"uuid":"550e8400-e29b-41d4-a716-446655440009"}`, rr.Body.String())
}

func TestHandleCreatePost_PassesTags(t *testing.T) {
	boardUUID := "550e8400-e29b-41d4-a716-446655440003"
	handler := newTestHandler(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{
			createPost: func(ctx context.Context, title, content string, tags []string, authorID int64, boardUUIDArg string) (string, error) {
				assert.Equal(t, "hello", title)
				assert.Equal(t, "body", content)
				assert.Equal(t, []string{"go", "backend"}, tags)
				assert.Equal(t, int64(1), authorID)
				assert.Equal(t, boardUUID, boardUUIDArg)
				return "550e8400-e29b-41d4-a716-446655440011", nil
			},
		},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{},
	)

	rr := doJSONRequestWithAuth(t, handler, http.MethodPost, "/boards/"+boardUUID+"/posts", map[string]any{
		"title":   "hello",
		"content": "body",
		"tags":    []string{"go", "backend"},
	}, 1)

	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.JSONEq(t, `{"uuid":"550e8400-e29b-41d4-a716-446655440011"}`, rr.Body.String())
}

func TestHandlePublishPost_Success(t *testing.T) {
	postUUID := "550e8400-e29b-41d4-a716-446655440005"
	handler := newTestHandler(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{
			publishPost: func(ctx context.Context, postUUIDArg string, authorID int64) error {
				assert.Equal(t, postUUID, postUUIDArg)
				assert.Equal(t, int64(1), authorID)
				return nil
			},
		},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{},
	)

	rr := doJSONRequestWithAuth(t, handler, http.MethodPost, "/posts/"+postUUID+"/publish", nil, 1)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestHandleCreateComment_WithParentUUID_Success(t *testing.T) {
	postUUID := "550e8400-e29b-41d4-a716-446655440003"
	parentUUID := "550e8400-e29b-41d4-a716-446655440009"
	handler := newTestHandler(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{},
		&fakeCommentUseCase{
			createComment: func(ctx context.Context, content string, authorID int64, postUUIDArg string, parentUUIDArg *string) (string, error) {
				assert.Equal(t, "reply", content)
				assert.Equal(t, int64(1), authorID)
				assert.Equal(t, postUUID, postUUIDArg)
				require.NotNil(t, parentUUIDArg)
				assert.Equal(t, parentUUID, *parentUUIDArg)
				return "550e8400-e29b-41d4-a716-446655440011", nil
			},
		},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{},
	)

	rr := doJSONRequestWithAuth(t, handler, http.MethodPost, "/posts/"+postUUID+"/comments", map[string]any{
		"content":     "reply",
		"parent_uuid": parentUUID,
	}, 1)

	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.JSONEq(t, `{"uuid":"550e8400-e29b-41d4-a716-446655440011"}`, rr.Body.String())
}

func TestHandleCreateComment_BadRequestForMalformedParentUUID(t *testing.T) {
	postUUID := "550e8400-e29b-41d4-a716-446655440003"
	handler := newTestHandler(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{},
		&fakeCommentUseCase{
			createComment: func(ctx context.Context, content string, authorID int64, postUUIDArg string, parentUUIDArg *string) (string, error) {
				t.Fatal("service should not be called for malformed parent_uuid")
				return "", nil
			},
		},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{},
	)

	rr := doJSONRequestWithAuth(t, handler, http.MethodPost, "/posts/"+postUUID+"/comments", map[string]any{
		"content":     "reply",
		"parent_uuid": "not-a-uuid",
	}, 1)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.JSONEq(t, `{"error":"invalid parent_uuid"}`, rr.Body.String())
}

func TestHandleGetAttachments_Success(t *testing.T) {
	postUUID := "550e8400-e29b-41d4-a716-446655440003"
	attachmentUUID := "550e8400-e29b-41d4-a716-446655440007"
	handler := newTestHandler(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{
			getPostAttachments: func(ctx context.Context, postUUIDArg string) ([]model.Attachment, error) {
				assert.Equal(t, postUUID, postUUIDArg)
				return []model.Attachment{{
					UUID:        attachmentUUID,
					PostUUID:    postUUID,
					FileName:    "a.png",
					ContentType: "image/png",
					SizeBytes:   10,
				}}, nil
			},
		},
	)

	rr := doJSONRequest(t, handler, http.MethodGet, "/posts/"+postUUID+"/attachments", nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.JSONEq(t, `{"attachments":[{"uuid":"550e8400-e29b-41d4-a716-446655440007","post_uuid":"550e8400-e29b-41d4-a716-446655440003","file_name":"a.png","content_type":"image/png","size_bytes":10,"file_url":"/api/v1/posts/550e8400-e29b-41d4-a716-446655440003/attachments/550e8400-e29b-41d4-a716-446655440007/file","preview_url":"/api/v1/posts/550e8400-e29b-41d4-a716-446655440003/attachments/550e8400-e29b-41d4-a716-446655440007/preview","created_at":"0001-01-01T00:00:00Z"}]}`, rr.Body.String())
}

func TestHandleDeleteAttachment_Success(t *testing.T) {
	postUUID := "550e8400-e29b-41d4-a716-446655440003"
	attachmentUUID := "550e8400-e29b-41d4-a716-446655440007"
	handler := newTestHandler(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{
			deletePostAttachment: func(ctx context.Context, postUUIDArg, attachmentUUIDArg string, userID int64) error {
				assert.Equal(t, postUUID, postUUIDArg)
				assert.Equal(t, attachmentUUID, attachmentUUIDArg)
				assert.Equal(t, int64(1), userID)
				return nil
			},
		},
	)

	rr := doJSONRequestWithAuth(t, handler, http.MethodDelete, "/posts/"+postUUID+"/attachments/"+attachmentUUID, nil, 1)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestHandleUploadAttachment_Success(t *testing.T) {
	postUUID := "550e8400-e29b-41d4-a716-446655440003"
	attachmentUUID := "550e8400-e29b-41d4-a716-446655440008"
	handler := newTestHandler(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{
			uploadPostAttachment: func(ctx context.Context, postUUIDArg string, userID int64, fileName, contentType string, content io.Reader) (*model.AttachmentUpload, error) {
				assert.Equal(t, postUUID, postUUIDArg)
				assert.Equal(t, int64(1), userID)
				assert.Equal(t, "a.png", fileName)
				assert.Equal(t, "image/png", contentType)
				data, err := io.ReadAll(content)
				require.NoError(t, err)
				assert.Equal(t, "hello", string(data))
				return &model.AttachmentUpload{UUID: attachmentUUID, EmbedMarkdown: "![a.png](attachment://" + attachmentUUID + ")"}, nil
			},
		},
	)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "a.png")
	require.NoError(t, err)
	_, err = io.WriteString(part, "hello")
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, apiV1Prefix+"/posts/"+postUUID+"/attachments/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	tokenProvider := auth.NewJwtTokenProvider("test-secret")
	token, err := tokenProvider.IdToToken(1)
	require.NoError(t, err)
	require.NoError(t, testSessionRepository.Save(context.Background(), 1, token, tokenProvider.TTLSeconds()))
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.JSONEq(t, `{"uuid":"550e8400-e29b-41d4-a716-446655440008","embed_markdown":"![a.png](attachment://550e8400-e29b-41d4-a716-446655440008)","preview_url":"/api/v1/posts/550e8400-e29b-41d4-a716-446655440003/attachments/550e8400-e29b-41d4-a716-446655440008/preview"}`, rr.Body.String())
}

func TestHandleUploadAttachment_RejectsOversizedMultipartBeforeUseCase(t *testing.T) {
	called := false
	user := &fakeUserUseCase{}
	tokenProvider := auth.NewJwtTokenProvider("test-secret")
	testSessionRepository = auth.NewCacheSessionRepository(cacheInMemory.NewInMemoryCache())
	sessionUseCase := service.NewSessionService(user, user, user, tokenProvider, testSessionRepository)
	handler := NewHTTPServer(":0", HTTPDependencies{
		SessionUseCase:  sessionUseCase,
		AdminAuthorizer: user,
		UserUseCase:     user,
		AccountUseCase:  &fakeAccountUseCase{},
		BoardUseCase:    &fakeBoardUseCase{},
		PostUseCase:     &fakePostUseCase{},
		CommentUseCase:  &fakeCommentUseCase{},
		ReactionUseCase: &fakeReactionUseCase{},
		AttachmentUseCase: &fakeAttachmentUseCase{
			uploadPostAttachment: func(ctx context.Context, postUUID string, userID int64, fileName, contentType string, content io.Reader) (*model.AttachmentUpload, error) {
				called = true
				return nil, nil
			},
		},
		AttachmentUploadMaxBytes: 4,
	}).Handler

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "a.png")
	require.NoError(t, err)
	_, err = part.Write(bytes.Repeat([]byte("a"), int(multipartRequestOverheadBytes*2)))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, apiV1Prefix+"/posts/550e8400-e29b-41d4-a716-446655440003/attachments/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	token, err := tokenProvider.IdToToken(1)
	require.NoError(t, err)
	require.NoError(t, testSessionRepository.Save(context.Background(), 1, token, tokenProvider.TTLSeconds()))
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.False(t, called)
}

func TestHandleGetAttachmentFile_Success(t *testing.T) {
	postUUID := "550e8400-e29b-41d4-a716-446655440003"
	attachmentUUID := "550e8400-e29b-41d4-a716-446655440008"
	handler := newTestHandler(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{
			getPostAttachmentFile: func(ctx context.Context, postUUIDArg, attachmentUUIDArg string) (*model.AttachmentFile, error) {
				assert.Equal(t, postUUID, postUUIDArg)
				assert.Equal(t, attachmentUUID, attachmentUUIDArg)
				return &model.AttachmentFile{
					FileName:    "a.png",
					ContentType: "image/png",
					SizeBytes:   5,
					ETag:        "\"att-8-5-0\"",
					Content:     io.NopCloser(strings.NewReader("hello")),
				}, nil
			},
		},
	)

	req := httptest.NewRequest(http.MethodGet, apiV1Prefix+"/posts/"+postUUID+"/attachments/"+attachmentUUID+"/file", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "image/png", rr.Header().Get("Content-Type"))
	assert.Equal(t, "no-store", rr.Header().Get("Cache-Control"))
	assert.Equal(t, "\"att-8-5-0\"", rr.Header().Get("ETag"))
	assert.Contains(t, rr.Header().Get("Content-Disposition"), "a.png")
	assert.Equal(t, "hello", rr.Body.String())
}

func TestHandleGetAttachmentFile_EscapesContentDispositionFilename(t *testing.T) {
	handler := newTestHandler(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{
			getPostAttachmentFile: func(ctx context.Context, postUUID, attachmentUUID string) (*model.AttachmentFile, error) {
				return &model.AttachmentFile{
					FileName:    "a\"b.png",
					ContentType: "image/png",
					SizeBytes:   5,
					Content:     io.NopCloser(strings.NewReader("hello")),
				}, nil
			},
		},
	)

	req := httptest.NewRequest(http.MethodGet, apiV1Prefix+"/posts/550e8400-e29b-41d4-a716-446655440003/attachments/550e8400-e29b-41d4-a716-446655440008/file", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	mediaType, params, err := mime.ParseMediaType(rr.Header().Get("Content-Disposition"))
	require.NoError(t, err)
	assert.Equal(t, "inline", mediaType)
	assert.Equal(t, "a\"b.png", params["filename"])
}

func TestHandleGetAttachmentFile_NotModifiedByETag(t *testing.T) {
	handler := newTestHandler(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{
			getPostAttachmentFile: func(ctx context.Context, postUUID, attachmentUUID string) (*model.AttachmentFile, error) {
				return &model.AttachmentFile{
					FileName:    "a.png",
					ContentType: "image/png",
					SizeBytes:   5,
					ETag:        "\"att-8-5-0\"",
					Content:     io.NopCloser(strings.NewReader("hello")),
				}, nil
			},
		},
	)

	req := httptest.NewRequest(http.MethodGet, apiV1Prefix+"/posts/550e8400-e29b-41d4-a716-446655440003/attachments/550e8400-e29b-41d4-a716-446655440008/file", nil)
	req.Header.Set("If-None-Match", "\"att-8-5-0\"")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotModified, rr.Code)
	assert.Empty(t, rr.Body.String())
	assert.Equal(t, "\"att-8-5-0\"", rr.Header().Get("ETag"))
}

func TestHandleGetAttachmentFile_NotFound(t *testing.T) {
	handler := newTestHandler(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{},
	)

	req := httptest.NewRequest(http.MethodGet, apiV1Prefix+"/posts/550e8400-e29b-41d4-a716-446655440003/attachments/550e8400-e29b-41d4-a716-446655440008/file", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.JSONEq(t, `{"error":"attachment not found"}`, rr.Body.String())
}

func TestHandleGetAttachmentPreview_Success(t *testing.T) {
	postUUID := "550e8400-e29b-41d4-a716-446655440003"
	attachmentUUID := "550e8400-e29b-41d4-a716-446655440008"
	handler := newTestHandler(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{
			getPostAttachmentPreviewFile: func(ctx context.Context, postUUIDArg, attachmentUUIDArg string, userID int64) (*model.AttachmentFile, error) {
				assert.Equal(t, postUUID, postUUIDArg)
				assert.Equal(t, attachmentUUID, attachmentUUIDArg)
				assert.Equal(t, int64(1), userID)
				return &model.AttachmentFile{
					FileName:    "a.png",
					ContentType: "image/png",
					SizeBytes:   5,
					ETag:        "\"att-8-5-0\"",
					Content:     io.NopCloser(strings.NewReader("hello")),
				}, nil
			},
		},
	)

	req := httptest.NewRequest(http.MethodGet, apiV1Prefix+"/posts/"+postUUID+"/attachments/"+attachmentUUID+"/preview", nil)
	tokenProvider := auth.NewJwtTokenProvider("test-secret")
	token, err := tokenProvider.IdToToken(1)
	require.NoError(t, err)
	require.NoError(t, testSessionRepository.Save(context.Background(), 1, token, tokenProvider.TTLSeconds()))
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "image/png", rr.Header().Get("Content-Type"))
	assert.Equal(t, "private, no-store", rr.Header().Get("Cache-Control"))
	assert.Contains(t, rr.Header().Get("Content-Disposition"), "a.png")
	assert.Equal(t, "hello", rr.Body.String())
}

func TestHandleGetAttachmentPreview_EscapesContentDispositionFilename(t *testing.T) {
	handler := newTestHandler(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{
			getPostAttachmentPreviewFile: func(ctx context.Context, postUUID, attachmentUUID string, userID int64) (*model.AttachmentFile, error) {
				return &model.AttachmentFile{
					FileName:    "a\"b.png",
					ContentType: "image/png",
					SizeBytes:   5,
					Content:     io.NopCloser(strings.NewReader("hello")),
				}, nil
			},
		},
	)

	req := httptest.NewRequest(http.MethodGet, apiV1Prefix+"/posts/550e8400-e29b-41d4-a716-446655440003/attachments/550e8400-e29b-41d4-a716-446655440008/preview", nil)
	tokenProvider := auth.NewJwtTokenProvider("test-secret")
	token, err := tokenProvider.IdToToken(1)
	require.NoError(t, err)
	require.NoError(t, testSessionRepository.Save(context.Background(), 1, token, tokenProvider.TTLSeconds()))
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	mediaType, params, err := mime.ParseMediaType(rr.Header().Get("Content-Disposition"))
	require.NoError(t, err)
	assert.Equal(t, "inline", mediaType)
	assert.Equal(t, "a\"b.png", params["filename"])
}

func TestHandleGetAttachmentPreview_NotFound(t *testing.T) {
	postUUID := "550e8400-e29b-41d4-a716-446655440003"
	attachmentUUID := "550e8400-e29b-41d4-a716-446655440008"
	handler := newTestHandler(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{},
	)

	req := httptest.NewRequest(http.MethodGet, apiV1Prefix+"/posts/"+postUUID+"/attachments/"+attachmentUUID+"/preview", nil)
	tokenProvider := auth.NewJwtTokenProvider("test-secret")
	token, err := tokenProvider.IdToToken(1)
	require.NoError(t, err)
	require.NoError(t, testSessionRepository.Save(context.Background(), 1, token, tokenProvider.TTLSeconds()))
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.JSONEq(t, `{"error":"attachment not found"}`, rr.Body.String())
}

func TestHandleUserSuspend_BadRequestForInvalidDuration(t *testing.T) {
	targetUserUUID := "550e8400-e29b-41d4-a716-446655440007"
	handler := newTestHandler(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{},
	)

	rr := doJSONRequestWithAuth(t, handler, http.MethodPut, "/users/"+targetUserUUID+"/suspension", map[string]any{
		"reason":   "spam",
		"duration": "3d",
	}, 1)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleUserUnsuspend_Success(t *testing.T) {
	expectedTargetUserUUID := "550e8400-e29b-41d4-a716-446655440007"
	handler := newTestHandler(
		&fakeUserUseCase{
			unsuspendUser: func(ctx context.Context, adminID int64, targetUserUUID string) error {
				assert.Equal(t, int64(1), adminID)
				assert.Equal(t, expectedTargetUserUUID, targetUserUUID)
				return nil
			},
		},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{},
	)

	rr := doJSONRequestWithAuth(t, handler, http.MethodDelete, "/users/"+expectedTargetUserUUID+"/suspension", nil, 1)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestHandleUserSuspend_BadRequestForMalformedUserUUID(t *testing.T) {
	handler := newTestHandler(
		&fakeUserUseCase{
			suspendUser: func(ctx context.Context, adminID int64, targetUserUUID, reason string, duration model.SuspensionDuration) error {
				t.Fatal("service should not be called for malformed user uuid")
				return nil
			},
		},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{},
	)

	rr := doJSONRequestWithAuth(t, handler, http.MethodPut, "/users/not-a-uuid/suspension", map[string]any{
		"reason":   "spam",
		"duration": "7d",
	}, 1)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.JSONEq(t, `{"error":"invalid user uuid"}`, rr.Body.String())
}

func TestHTTP_UserSignUp_MethodNotAllowed(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/signup", nil)
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
	assert.JSONEq(t, `{"error":"method not allowed"}`, rr.Body.String())
}

func TestHTTP_UserSignUp_Conflict(t *testing.T) {
	user := &fakeUserUseCase{
		signUp: func(ctx context.Context, username, password string) (string, error) {
			return "", customerror.ErrUserAlreadyExists
		},
	}
	handler := newTestHandler(user, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequest(t, handler, http.MethodPost, "/signup", map[string]string{
		"username": "alice",
		"password": "pw",
	})
	assert.Equal(t, http.StatusConflict, rr.Code)
}

func TestHTTP_UserLogin_Success(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequest(t, handler, http.MethodPost, "/auth/login", map[string]string{
		"username": "alice",
		"password": "pw",
	})

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Header().Get("Authorization"), "Bearer ")
}

func TestHTTP_GuestIssue_Success(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{
		issueGuestAccount: func(ctx context.Context) (int64, error) {
			return 77, nil
		},
	}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequest(t, handler, http.MethodPost, "/auth/guest", nil)

	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.Contains(t, rr.Header().Get("Authorization"), "Bearer ")
	assert.JSONEq(t, `{"login":"ok"}`, rr.Body.String())
}

func TestHTTP_GuestUpgrade_Success(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{
		upgradeGuest: func(ctx context.Context, userID int64, username, email, password string) error {
			assert.Equal(t, int64(1), userID)
			assert.Equal(t, "alice", username)
			assert.Equal(t, "alice@example.com", email)
			assert.Equal(t, "pw", password)
			return nil
		},
	}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})
	tokenProvider := auth.NewJwtTokenProvider("test-secret")
	oldToken, err := tokenProvider.IdToToken(1)
	require.NoError(t, err)
	require.NotNil(t, testSessionRepository)
	require.NoError(t, testSessionRepository.Save(context.Background(), 1, oldToken, tokenProvider.TTLSeconds()))

	rr := doJSONRequestWithAuth(t, handler, http.MethodPost, "/auth/guest/upgrade", map[string]any{
		"username": "alice",
		"email":    "alice@example.com",
		"password": "pw",
	}, 1, withAuthToken(oldToken))

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Header().Get("Authorization"), "Bearer ")
	assert.JSONEq(t, `{"result":"ok"}`, rr.Body.String())
}

func TestHTTP_GuestUpgrade_BadRequest_WhenEmailMissing(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequestWithAuth(t, handler, http.MethodPost, "/auth/guest/upgrade", map[string]any{
		"username": "alice",
		"password": "pw",
	}, 1)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.JSONEq(t, `{"error":"username, email and password are required"}`, rr.Body.String())
}

func TestHTTP_UserLogin_BadRequest_WhenUsernameMissing(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequest(t, handler, http.MethodPost, "/auth/login", map[string]string{
		"password": "pw",
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.JSONEq(t, `{"error":"username and password are required"}`, rr.Body.String())
}

func TestHTTP_UserLogout_Success(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequestWithAuth(t, handler, http.MethodPost, "/auth/logout", map[string]any{}, 1)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.JSONEq(t, `{"logout":"ok"}`, rr.Body.String())
}

func TestHTTP_PostReactionMeCreate_Created(t *testing.T) {
	postUUID := "550e8400-e29b-41d4-a716-446655440001"
	reaction := &fakeReactionUseCase{
		setReaction: func(ctx context.Context, userID int64, targetUUID string, targetType model.ReactionTargetType, reactionType model.ReactionType) (bool, error) {
			assert.Equal(t, postUUID, targetUUID)
			return true, nil
		},
	}
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, reaction, &fakeAttachmentUseCase{})

	rr := doJSONRequestWithAuth(t, handler, http.MethodPut, "/posts/"+postUUID+"/reactions/me", map[string]string{
		"reaction_type": "like",
	}, 1)
	assert.Equal(t, http.StatusCreated, rr.Code)
}

func TestHTTP_CommentReactionMeUpdate_NoContent(t *testing.T) {
	commentUUID := "550e8400-e29b-41d4-a716-446655440001"
	reaction := &fakeReactionUseCase{
		setReaction: func(ctx context.Context, userID int64, targetUUID string, targetType model.ReactionTargetType, reactionType model.ReactionType) (bool, error) {
			assert.Equal(t, commentUUID, targetUUID)
			return false, nil
		},
	}
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, reaction, &fakeAttachmentUseCase{})

	rr := doJSONRequestWithAuth(t, handler, http.MethodPut, "/comments/"+commentUUID+"/reactions/me", map[string]string{
		"reaction_type": "dislike",
	}, 1)
	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestHTTP_PostReactionMeDelete_NoContent(t *testing.T) {
	postUUID := "550e8400-e29b-41d4-a716-446655440001"
	reaction := &fakeReactionUseCase{
		deleteReaction: func(ctx context.Context, userID int64, targetUUID string, targetType model.ReactionTargetType) error {
			assert.Equal(t, postUUID, targetUUID)
			return nil
		},
	}
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, reaction, &fakeAttachmentUseCase{})

	rr := doJSONRequestWithAuth(t, handler, http.MethodDelete, "/posts/"+postUUID+"/reactions/me", nil, 1)
	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestHTTP_UserSignUp_BadJSON(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	req := httptest.NewRequest(http.MethodPost, apiV1Prefix+"/signup", bytes.NewBufferString("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHTTP_UserSignUp_UnknownField(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequest(t, handler, http.MethodPost, "/signup", map[string]any{
		"username": "alice",
		"password": "pw",
		"extra":    "unknown",
	})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHTTP_UserSignUp_TrailingJSONRejected(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	req := httptest.NewRequest(http.MethodPost, apiV1Prefix+"/signup", bytes.NewBufferString(`{"username":"alice","password":"pw"}{"extra":true}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHTTP_UserSignUp_OversizedJSONRejected(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	hugeUsername := strings.Repeat("a", int(defaultMaxJSONBodyBytes))
	body := `{"username":"` + hugeUsername + `","password":"pw"}`
	req := httptest.NewRequest(http.MethodPost, apiV1Prefix+"/signup", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "request body too large")
}

func TestHTTP_UserSignUp_OversizedJSONRejected_WithConfiguredLimit(t *testing.T) {
	handler := newTestHandlerWithJSONLimit(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{},
		80,
	)

	req := httptest.NewRequest(http.MethodPost, apiV1Prefix+"/signup", bytes.NewBufferString(`{"username":"alice","password":"`+strings.Repeat("p", 128)+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "request body too large")
}

func TestHTTP_WriteRateLimit_ReturnsTooManyRequests(t *testing.T) {
	handler := newTestHandlerWithRateLimit(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{},
		true,
		60,
		50,
		2,
	)

	reqBody := map[string]any{
		"username": "alice",
		"password": "pw",
	}
	first := doJSONRequest(t, handler, http.MethodPost, "/signup", reqBody)
	assert.Equal(t, http.StatusCreated, first.Code)
	second := doJSONRequest(t, handler, http.MethodPost, "/signup", reqBody)
	assert.Equal(t, http.StatusCreated, second.Code)
	third := doJSONRequest(t, handler, http.MethodPost, "/signup", reqBody)
	assert.Equal(t, http.StatusTooManyRequests, third.Code)
	assert.JSONEq(t, `{"error":"too many requests"}`, third.Body.String())
}

func TestHTTP_ReadRateLimit_ReturnsTooManyRequests(t *testing.T) {
	handler := newTestHandlerWithRateLimit(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{},
		true,
		60,
		2,
		10,
	)

	first := doJSONRequest(t, handler, http.MethodGet, "/boards", nil)
	assert.Equal(t, http.StatusOK, first.Code)
	second := doJSONRequest(t, handler, http.MethodGet, "/boards", nil)
	assert.Equal(t, http.StatusOK, second.Code)
	third := doJSONRequest(t, handler, http.MethodGet, "/boards", nil)
	assert.Equal(t, http.StatusTooManyRequests, third.Code)
	assert.JSONEq(t, `{"error":"too many requests"}`, third.Body.String())
}

func TestHTTP_RateLimit_DoesNotTrustForwardedHeaderByDefault(t *testing.T) {
	handler := newTestHandlerWithRateLimit(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{},
		true,
		60,
		1,
		10,
	)

	first := httptest.NewRequest(http.MethodGet, apiV1Prefix+"/boards", nil)
	first.RemoteAddr = "198.51.100.10:1234"
	first.Header.Set("X-Forwarded-For", "203.0.113.1")
	firstRec := httptest.NewRecorder()
	handler.ServeHTTP(firstRec, first)
	assert.Equal(t, http.StatusOK, firstRec.Code)

	second := httptest.NewRequest(http.MethodGet, apiV1Prefix+"/boards", nil)
	second.RemoteAddr = "198.51.100.10:9999"
	second.Header.Set("X-Forwarded-For", "203.0.113.99")
	secondRec := httptest.NewRecorder()
	handler.ServeHTTP(secondRec, second)
	assert.Equal(t, http.StatusTooManyRequests, secondRec.Code)
	assert.JSONEq(t, `{"error":"too many requests"}`, secondRec.Body.String())
}

func TestHTTP_BoardPostsPost_PreservesRawMarkdownInput(t *testing.T) {
	boardUUID := "550e8400-e29b-41d4-a716-446655440003"
	post := &fakePostUseCase{
		createPost: func(ctx context.Context, title, content string, tags []string, authorID int64, boardUUIDArg string) (string, error) {
			assert.Equal(t, "hello <script>alert(1)</script>", title)
			assert.Equal(t, "body <img src=x onerror=alert(1)> `code` ![a](attachment://550e8400-e29b-41d4-a716-446655440003)", content)
			assert.Equal(t, int64(1), authorID)
			assert.Equal(t, boardUUID, boardUUIDArg)
			return "550e8400-e29b-41d4-a716-446655440010", nil
		},
	}
	handler := newTestHandler(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		post,
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{},
	)
	rr := doJSONRequestWithAuth(t, handler, http.MethodPost, "/boards/"+boardUUID+"/posts", map[string]any{
		"title":   "hello <script>alert(1)</script>",
		"content": "body <img src=x onerror=alert(1)> `code` ![a](attachment://550e8400-e29b-41d4-a716-446655440003)",
		"tags":    []string{"go"},
	}, 1)
	assert.Equal(t, http.StatusCreated, rr.Code)
}

func TestHTTP_UserDeleteMe_Unauthorized(t *testing.T) {
	account := &fakeAccountUseCase{
		deleteMyAccount: func(ctx context.Context, userID int64, password string) error {
			return customerror.ErrInvalidCredential
		},
	}
	handler := newTestHandler(&fakeUserUseCase{}, account, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequestWithAuth(t, handler, http.MethodDelete, "/users/me", map[string]string{
		"password": "wrong",
	}, 1)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestHTTP_ProtectedRoute_InvalidAuthorizationScheme(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	req := httptest.NewRequest(http.MethodPost, apiV1Prefix+"/auth/logout", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "token-only")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.JSONEq(t, `{"error":"invalid token"}`, rr.Body.String())
}

func TestHTTP_BoardCreate_Forbidden(t *testing.T) {
	board := &fakeBoardUseCase{
		createBoard: func(ctx context.Context, userID int64, name, description string) (string, error) {
			return "", customerror.ErrForbidden
		},
	}
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, board, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequestWithAuth(t, handler, http.MethodPost, "/boards", map[string]any{
		"name":        "free",
		"description": "desc",
	}, 2)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestHTTP_BoardDelete_Success(t *testing.T) {
	boardUUID := "550e8400-e29b-41d4-a716-446655440003"
	board := &fakeBoardUseCase{
		deleteBoard: func(ctx context.Context, boardUUIDArg string, userID int64) error {
			assert.Equal(t, boardUUID, boardUUIDArg)
			assert.Equal(t, int64(1), userID)
			return nil
		},
	}
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, board, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequestWithAuth(t, handler, http.MethodDelete, "/boards/"+boardUUID, nil, 1)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestHTTP_BoardPostsGet_Success(t *testing.T) {
	boardUUID := "550e8400-e29b-41d4-a716-446655440003"
	postUUID := "550e8400-e29b-41d4-a716-446655440004"
	post := &fakePostUseCase{
		getPostsList: func(ctx context.Context, boardUUIDArg string, limit int, cursor string) (*model.PostList, error) {
			assert.Equal(t, boardUUID, boardUUIDArg)
			assert.Equal(t, 2, limit)
			assert.Equal(t, "opaque-cursor-9", cursor)
			return &model.PostList{
				Posts: []model.Post{{UUID: postUUID, Title: "hello", Content: "world", AuthorUUID: "user-uuid", BoardUUID: boardUUID}},
				Limit: limit, Cursor: cursor, HasMore: false,
			}, nil
		},
	}
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, post, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	req := httptest.NewRequest(http.MethodGet, apiV1Prefix+"/boards/"+boardUUID+"/posts?limit=2&cursor=opaque-cursor-9", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"author_uuid":"user-uuid"`)
}

func TestHTTP_PostDetailPut_Success(t *testing.T) {
	postUUID := "550e8400-e29b-41d4-a716-446655440003"
	post := &fakePostUseCase{
		updatePost: func(ctx context.Context, postUUIDArg string, authorID int64, title, content string, tags []string) error {
			assert.Equal(t, postUUID, postUUIDArg)
			assert.Equal(t, int64(1), authorID)
			assert.Equal(t, "hello", title)
			assert.Equal(t, "world", content)
			assert.Equal(t, []string{"go"}, tags)
			return nil
		},
	}
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, post, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequestWithAuth(t, handler, http.MethodPut, "/posts/"+postUUID, map[string]any{
		"title":   "hello",
		"content": "world",
		"tags":    []string{"go"},
	}, 1)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestHTTP_PostDetailDelete_Success(t *testing.T) {
	postUUID := "550e8400-e29b-41d4-a716-446655440003"
	post := &fakePostUseCase{
		deletePost: func(ctx context.Context, postUUIDArg string, authorID int64) error {
			assert.Equal(t, postUUID, postUUIDArg)
			assert.Equal(t, int64(1), authorID)
			return nil
		},
	}
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, post, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequestWithAuth(t, handler, http.MethodDelete, "/posts/"+postUUID, nil, 1)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestHTTP_PostCommentsGet_Success(t *testing.T) {
	postUUID := "550e8400-e29b-41d4-a716-446655440003"
	commentUUID := "550e8400-e29b-41d4-a716-446655440004"
	comment := &fakeCommentUseCase{
		getCommentsByPost: func(ctx context.Context, postUUIDArg string, limit int, cursor string) (*model.CommentList, error) {
			assert.Equal(t, postUUID, postUUIDArg)
			assert.Equal(t, 2, limit)
			assert.Equal(t, "opaque-cursor-9", cursor)
			return &model.CommentList{
				Comments: []model.Comment{{UUID: commentUUID, Content: "nice", AuthorUUID: "user-uuid", PostUUID: postUUID}},
				Limit:    limit, Cursor: cursor,
			}, nil
		},
	}
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, comment, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	req := httptest.NewRequest(http.MethodGet, apiV1Prefix+"/posts/"+postUUID+"/comments?limit=2&cursor=opaque-cursor-9", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"author_uuid":"user-uuid"`)
}

func TestHTTP_CommentPut_Success(t *testing.T) {
	commentUUID := "550e8400-e29b-41d4-a716-446655440003"
	comment := &fakeCommentUseCase{
		updateComment: func(ctx context.Context, commentUUIDArg string, authorID int64, content string) error {
			assert.Equal(t, commentUUID, commentUUIDArg)
			assert.Equal(t, int64(1), authorID)
			assert.Equal(t, "updated", content)
			return nil
		},
	}
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, comment, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequestWithAuth(t, handler, http.MethodPut, "/comments/"+commentUUID, map[string]any{
		"content": "updated",
	}, 1)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestHTTP_CommentDelete_Success(t *testing.T) {
	commentUUID := "550e8400-e29b-41d4-a716-446655440003"
	comment := &fakeCommentUseCase{
		deleteComment: func(ctx context.Context, commentUUIDArg string, authorID int64) error {
			assert.Equal(t, commentUUID, commentUUIDArg)
			assert.Equal(t, int64(1), authorID)
			return nil
		},
	}
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, comment, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequestWithAuth(t, handler, http.MethodDelete, "/comments/"+commentUUID, nil, 1)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestHTTP_BoardGet_BadLimit(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/boards?limit=bad", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	rr = doJSONRequest(t, handler, http.MethodGet, "/boards?limit=0", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	rr = doJSONRequest(t, handler, http.MethodGet, "/boards?limit=1001", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHTTP_BoardGet_UsesConfiguredDefaultPageLimit(t *testing.T) {
	handler := newTestHandlerWithDefaultPageLimit(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{
			getBoards: func(ctx context.Context, limit int, cursor string) (*model.BoardList, error) {
				assert.Equal(t, 7, limit)
				assert.Equal(t, "", cursor)
				return &model.BoardList{Boards: []model.Board{}, Limit: limit, Cursor: cursor}, nil
			},
		},
		&fakePostUseCase{},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{},
		7,
	)

	rr := doJSONRequest(t, handler, http.MethodGet, "/boards", nil)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHTTP_BoardGet_PassesOpaqueCursor(t *testing.T) {
	handler := newTestHandler(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{
			getBoards: func(ctx context.Context, limit int, cursor string) (*model.BoardList, error) {
				assert.Equal(t, "bad", cursor)
				return &model.BoardList{Boards: []model.Board{}, Limit: limit, Cursor: cursor}, nil
			},
		},
		&fakePostUseCase{},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{},
	)

	rr := doJSONRequest(t, handler, http.MethodGet, "/boards?cursor=bad", nil)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHTTP_BoardWithID_InvalidBoardUUID(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequestWithAuth(t, handler, http.MethodPut, "/boards/abc", map[string]any{
		"name": "free",
	}, 1)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHTTP_BoardWithID_UnauthorizedBeforeValidation(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequest(t, handler, http.MethodPut, "/boards/abc", map[string]any{
		"name": "free",
	})
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestHTTP_PostWithID_InvalidPostUUID(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/posts/abc", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHTTP_PostWithID_InvalidUUIDLikeValue(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/posts/0", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	rr = doJSONRequest(t, handler, http.MethodGet, "/posts/not-a-uuid", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHTTP_ReactionDelete_BadUserID(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequest(t, handler, http.MethodDelete, "/posts/550e8400-e29b-41d4-a716-446655440001/reactions/me", nil)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestHTTP_PostReactionList_InvalidPostID(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/posts/abc/reactions", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHTTP_CommentReactionList_InvalidCommentID(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/comments/abc/reactions", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHTTP_ReactionWithID_MethodNotAllowed(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/posts/550e8400-e29b-41d4-a716-446655440001/reactions/me", nil)
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
	assert.JSONEq(t, `{"error":"method not allowed"}`, rr.Body.String())
}

func TestHTTP_PostDetail_InternalServerErrorFallback(t *testing.T) {
	post := &fakePostUseCase{
		getPostDetail: func(ctx context.Context, postUUID string) (*model.PostDetail, error) {
			return nil, errors.New("unexpected")
		},
	}
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, post, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/posts/550e8400-e29b-41d4-a716-446655440010", nil)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestHTTP_PostDetail_NotFound(t *testing.T) {
	post := &fakePostUseCase{
		getPostDetail: func(ctx context.Context, postUUID string) (*model.PostDetail, error) {
			return nil, customerror.ErrPostNotFound
		},
	}
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, post, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/posts/550e8400-e29b-41d4-a716-446655440010", nil)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHTTP_PostDetail_IncludesCommentsHasMore(t *testing.T) {
	post := &fakePostUseCase{
		getPostDetail: func(ctx context.Context, postUUID string) (*model.PostDetail, error) {
			return &model.PostDetail{
				Post:            &model.Post{UUID: postUUID, Title: "title"},
				CommentsHasMore: true,
			}, nil
		},
	}
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, post, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/posts/550e8400-e29b-41d4-a716-446655440010", nil)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"comments_has_more":true`)
}

func TestHTTP_PostDetail_IncludesTags(t *testing.T) {
	post := &fakePostUseCase{
		getPostDetail: func(ctx context.Context, postUUID string) (*model.PostDetail, error) {
			return &model.PostDetail{
				Post: &model.Post{UUID: postUUID, Title: "hello"},
				Tags: []model.Tag{{ID: 1, Name: "go"}},
			}, nil
		},
	}
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, post, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/posts/550e8400-e29b-41d4-a716-446655440010", nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"tags":[{"id":1,"name":"go"`)
}

func TestHTTP_TagPosts_Success(t *testing.T) {
	post := &fakePostUseCase{
		getPostsByTag: func(ctx context.Context, tagName string, limit int, cursor string) (*model.PostList, error) {
			assert.Equal(t, "go", tagName)
			assert.Equal(t, 10, limit)
			assert.Equal(t, "", cursor)
			return &model.PostList{
				Posts: []model.Post{{UUID: "550e8400-e29b-41d4-a716-446655440003", Title: "hello"}},
				Limit: limit,
			}, nil
		},
	}
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, post, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/tags/go/posts?limit=10", nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"posts":[{"uuid":"550e8400-e29b-41d4-a716-446655440003"`)
}

func TestHTTP_NotFound(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/unknown", nil)
	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.JSONEq(t, `{"error":"not found"}`, rr.Body.String())
}

func TestHTTP_NotFound_UsesInjectedLogger(t *testing.T) {
	user := &fakeUserUseCase{}
	tokenProvider := auth.NewJwtTokenProvider("test-secret")
	testSessionRepository = auth.NewCacheSessionRepository(cacheInMemory.NewInMemoryCache())
	sessionUseCase := service.NewSessionService(user, user, user, tokenProvider, testSessionRepository)
	logger := &spyLogger{}
	handler := NewHTTPServer(":0", HTTPDependencies{
		SessionUseCase:    sessionUseCase,
		AdminAuthorizer:   user,
		UserUseCase:       user,
		AccountUseCase:    &fakeAccountUseCase{},
		BoardUseCase:      &fakeBoardUseCase{},
		PostUseCase:       &fakePostUseCase{},
		CommentUseCase:    &fakeCommentUseCase{},
		ReactionUseCase:   &fakeReactionUseCase{},
		AttachmentUseCase: &fakeAttachmentUseCase{},
		Logger:            logger.Logger(),
	}).Handler

	rr := doJSONRequest(t, handler, http.MethodGet, "/unknown", nil)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.Equal(t, 1, logger.warns)
	assert.Equal(t, 0, logger.errors)
}
