package delivery

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/application/service"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/hoonzinope/go-comu-bin/internal/infrastructure/auth"
	cacheInMemory "github.com/hoonzinope/go-comu-bin/internal/infrastructure/cache/inmemory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const apiV1Prefix = "/api/v1"

type fakeUserUseCase struct {
	signUp            func(username, password string) (string, error)
	deleteMe          func(userID int64, password string) error
	getUserSuspension func(adminID int64, targetUserUUID string) (*model.UserSuspension, error)
	suspendUser       func(adminID int64, targetUserUUID, reason string, duration entity.SuspensionDuration) error
	unsuspendUser     func(adminID int64, targetUserUUID string) error
	verifyCredential  func(username, password string) (int64, error)
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

func (f *fakeUserUseCase) GetUserSuspension(adminID int64, targetUserUUID string) (*model.UserSuspension, error) {
	if f.getUserSuspension != nil {
		return f.getUserSuspension(adminID, targetUserUUID)
	}
	return &model.UserSuspension{}, nil
}

func (f *fakeUserUseCase) SuspendUser(adminID int64, targetUserUUID, reason string, duration entity.SuspensionDuration) error {
	if f.suspendUser != nil {
		return f.suspendUser(adminID, targetUserUUID, reason, duration)
	}
	return nil
}

func (f *fakeUserUseCase) UnsuspendUser(adminID int64, targetUserUUID string) error {
	if f.unsuspendUser != nil {
		return f.unsuspendUser(adminID, targetUserUUID)
	}
	return nil
}

func (f *fakeUserUseCase) VerifyCredentials(username, password string) (int64, error) {
	if f.verifyCredential != nil {
		return f.verifyCredential(username, password)
	}
	return 1, nil
}

func (f *fakeUserUseCase) Save(user *entity.User) (int64, error) {
	return 1, nil
}

func (f *fakeUserUseCase) SelectUserByUsername(username string) (*entity.User, error) {
	return &entity.User{ID: 1, Name: username, Status: entity.UserStatusActive}, nil
}

func (f *fakeUserUseCase) SelectUserByUUID(userUUID string) (*entity.User, error) {
	return &entity.User{ID: 1, UUID: userUUID, Status: entity.UserStatusActive}, nil
}

func (f *fakeUserUseCase) SelectUserByID(id int64) (*entity.User, error) {
	return &entity.User{ID: id, Name: "user", Status: entity.UserStatusActive}, nil
}

func (f *fakeUserUseCase) SelectUserByIDIncludingDeleted(id int64) (*entity.User, error) {
	return &entity.User{ID: id, Name: "user", Status: entity.UserStatusActive}, nil
}

func (f *fakeUserUseCase) SelectUsersByIDsIncludingDeleted(ids []int64) (map[int64]*entity.User, error) {
	out := make(map[int64]*entity.User, len(ids))
	for _, id := range ids {
		out[id] = &entity.User{ID: id, Name: "user", Status: entity.UserStatusActive}
	}
	return out, nil
}

func (f *fakeUserUseCase) Update(user *entity.User) error {
	return nil
}

func (f *fakeUserUseCase) Delete(id int64) error {
	return nil
}

type fakeAccountUseCase struct {
	deleteMyAccount func(userID int64, password string) error
}

func (f *fakeAccountUseCase) DeleteMyAccount(userID int64, password string) error {
	if f.deleteMyAccount != nil {
		return f.deleteMyAccount(userID, password)
	}
	return nil
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
	createPost      func(title, content string, tags []string, authorID, boardID int64) (int64, error)
	createDraftPost func(title, content string, tags []string, authorID, boardID int64) (int64, error)
	getPostsList    func(boardID int64, limit int, lastID int64) (*model.PostList, error)
	getPostsByTag   func(tagName string, limit int, lastID int64) (*model.PostList, error)
	getPostDetail   func(postID int64) (*model.PostDetail, error)
	publishPost     func(id, authorID int64) error
	updatePost      func(id, authorID int64, title, content string, tags []string) error
	deletePost      func(id, authorID int64) error
}

func (f *fakePostUseCase) CreatePost(title, content string, tags []string, authorID, boardID int64) (int64, error) {
	if f.createPost != nil {
		return f.createPost(title, content, tags, authorID, boardID)
	}
	return 1, nil
}

func (f *fakePostUseCase) CreateDraftPost(title, content string, tags []string, authorID, boardID int64) (int64, error) {
	if f.createDraftPost != nil {
		return f.createDraftPost(title, content, tags, authorID, boardID)
	}
	return 1, nil
}

func (f *fakePostUseCase) GetPostsList(boardID int64, limit int, lastID int64) (*model.PostList, error) {
	if f.getPostsList != nil {
		return f.getPostsList(boardID, limit, lastID)
	}
	return &model.PostList{}, nil
}

func (f *fakePostUseCase) GetPostsByTag(tagName string, limit int, lastID int64) (*model.PostList, error) {
	if f.getPostsByTag != nil {
		return f.getPostsByTag(tagName, limit, lastID)
	}
	return &model.PostList{}, nil
}

func (f *fakePostUseCase) GetPostDetail(postID int64) (*model.PostDetail, error) {
	if f.getPostDetail != nil {
		return f.getPostDetail(postID)
	}
	return &model.PostDetail{}, nil
}

func (f *fakePostUseCase) PublishPost(id, authorID int64) error {
	if f.publishPost != nil {
		return f.publishPost(id, authorID)
	}
	return nil
}

func (f *fakePostUseCase) UpdatePost(id, authorID int64, title, content string, tags []string) error {
	if f.updatePost != nil {
		return f.updatePost(id, authorID, title, content, tags)
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
	createComment     func(content string, authorID, postID int64, parentID *int64) (int64, error)
	getCommentsByPost func(postID int64, limit int, lastID int64) (*model.CommentList, error)
	updateComment     func(id, authorID int64, content string) error
	deleteComment     func(id, authorID int64) error
}

func (f *fakeCommentUseCase) CreateComment(content string, authorID, postID int64, parentID *int64) (int64, error) {
	if f.createComment != nil {
		return f.createComment(content, authorID, postID, parentID)
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
	setReaction          func(userID, targetID int64, targetType entity.ReactionTargetType, reactionType entity.ReactionType) (bool, error)
	deleteReaction       func(userID, targetID int64, targetType entity.ReactionTargetType) error
	getReactionsByTarget func(targetID int64, targetType entity.ReactionTargetType) ([]model.Reaction, error)
}

type fakeAttachmentUseCase struct {
	createPostAttachment         func(postID, userID int64, fileName, contentType string, sizeBytes int64, storageKey string) (int64, error)
	getPostAttachments           func(postID int64) ([]model.Attachment, error)
	getPostAttachmentFile        func(postID, attachmentID int64) (*model.AttachmentFile, error)
	getPostAttachmentPreviewFile func(postID, attachmentID, userID int64) (*model.AttachmentFile, error)
	deletePostAttachment         func(postID, attachmentID, userID int64) error
	uploadPostAttachment         func(postID, userID int64, fileName, contentType string, content io.Reader) (*model.AttachmentUpload, error)
}

func (f *fakeAttachmentUseCase) CreatePostAttachment(postID, userID int64, fileName, contentType string, sizeBytes int64, storageKey string) (int64, error) {
	if f.createPostAttachment != nil {
		return f.createPostAttachment(postID, userID, fileName, contentType, sizeBytes, storageKey)
	}
	return 1, nil
}

func (f *fakeAttachmentUseCase) GetPostAttachments(postID int64) ([]model.Attachment, error) {
	if f.getPostAttachments != nil {
		return f.getPostAttachments(postID)
	}
	return []model.Attachment{}, nil
}

func (f *fakeAttachmentUseCase) GetPostAttachmentFile(postID, attachmentID int64) (*model.AttachmentFile, error) {
	if f.getPostAttachmentFile != nil {
		return f.getPostAttachmentFile(postID, attachmentID)
	}
	return nil, customError.ErrAttachmentNotFound
}

func (f *fakeAttachmentUseCase) GetPostAttachmentPreviewFile(postID, attachmentID, userID int64) (*model.AttachmentFile, error) {
	if f.getPostAttachmentPreviewFile != nil {
		return f.getPostAttachmentPreviewFile(postID, attachmentID, userID)
	}
	return nil, customError.ErrAttachmentNotFound
}

func (f *fakeAttachmentUseCase) DeletePostAttachment(postID, attachmentID, userID int64) error {
	if f.deletePostAttachment != nil {
		return f.deletePostAttachment(postID, attachmentID, userID)
	}
	return nil
}

func (f *fakeAttachmentUseCase) UploadPostAttachment(postID, userID int64, fileName, contentType string, content io.Reader) (*model.AttachmentUpload, error) {
	if f.uploadPostAttachment != nil {
		return f.uploadPostAttachment(postID, userID, fileName, contentType, content)
	}
	return &model.AttachmentUpload{ID: 1, EmbedMarkdown: "![a.png](attachment://1)"}, nil
}

var testSessionRepository port.SessionRepository

type authUserPort interface {
	port.UserUseCase
	port.CredentialVerifier
	port.UserRepository
}

func (f *fakeReactionUseCase) SetReaction(userID, targetID int64, targetType entity.ReactionTargetType, reactionType entity.ReactionType) (bool, error) {
	if f.setReaction != nil {
		return f.setReaction(userID, targetID, targetType, reactionType)
	}
	return false, nil
}

func (f *fakeReactionUseCase) DeleteReaction(userID, targetID int64, targetType entity.ReactionTargetType) error {
	if f.deleteReaction != nil {
		return f.deleteReaction(userID, targetID, targetType)
	}
	return nil
}

func (f *fakeReactionUseCase) GetReactionsByTarget(targetID int64, targetType entity.ReactionTargetType) ([]model.Reaction, error) {
	if f.getReactionsByTarget != nil {
		return f.getReactionsByTarget(targetID, targetType)
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
	sessionUseCase := service.NewSessionService(user, user, tokenProvider, testSessionRepository)
	return NewHTTPServer(":0", HTTPDependencies{
		SessionUseCase:    sessionUseCase,
		UserUseCase:       user,
		AccountUseCase:    account,
		BoardUseCase:      board,
		PostUseCase:       post,
		CommentUseCase:    comment,
		ReactionUseCase:   reaction,
		AttachmentUseCase: attachment,
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
	tokenProvider := auth.NewJwtTokenProvider("test-secret")
	token, err := tokenProvider.IdToToken(userID)
	require.NoError(t, err)
	require.NotNil(t, testSessionRepository)
	require.NoError(t, testSessionRepository.Save(userID, token, tokenProvider.TTLSeconds()))

	req := httptest.NewRequest(method, apiV1Prefix+path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func TestHandleUserSuspend_Success(t *testing.T) {
	handler := newTestHandler(
		&fakeUserUseCase{
			suspendUser: func(adminID int64, targetUserUUID, reason string, duration entity.SuspensionDuration) error {
				assert.Equal(t, int64(1), adminID)
				assert.Equal(t, "user-uuid-7", targetUserUUID)
				assert.Equal(t, "spam", reason)
				assert.Equal(t, entity.SuspensionDuration7Days, duration)
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

	rr := doJSONRequestWithAuth(t, handler, http.MethodPut, "/users/user-uuid-7/suspension", map[string]any{
		"reason":   "spam",
		"duration": "7d",
	}, 1)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestHandleUserSuspensionGet_Success(t *testing.T) {
	until := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	handler := newTestHandler(
		&fakeUserUseCase{
			getUserSuspension: func(adminID int64, targetUserUUID string) (*model.UserSuspension, error) {
				assert.Equal(t, int64(1), adminID)
				assert.Equal(t, "user-uuid-7", targetUserUUID)
				return &model.UserSuspension{
					UserUUID:       "user-uuid-7",
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

	rr := doJSONRequestWithAuth(t, handler, http.MethodGet, "/users/user-uuid-7/suspension", nil, 1)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.JSONEq(t, `{
		"user_uuid": "user-uuid-7",
		"status": "suspended",
		"reason": "spam",
		"suspended_until": "2026-03-15T10:00:00Z"
	}`, rr.Body.String())
}

func TestHandleCreateDraftPost_Success(t *testing.T) {
	handler := newTestHandler(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{
			createDraftPost: func(title, content string, tags []string, authorID, boardID int64) (int64, error) {
				assert.Equal(t, "draft", title)
				assert.Equal(t, "content", content)
				assert.Nil(t, tags)
				assert.Equal(t, int64(1), authorID)
				assert.Equal(t, int64(3), boardID)
				return 9, nil
			},
		},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{},
	)

	rr := doJSONRequestWithAuth(t, handler, http.MethodPost, "/boards/3/posts/drafts", map[string]string{
		"title":   "draft",
		"content": "content",
	}, 1)

	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.JSONEq(t, `{"id":9}`, rr.Body.String())
}

func TestHandleCreatePost_PassesTags(t *testing.T) {
	handler := newTestHandler(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{
			createPost: func(title, content string, tags []string, authorID, boardID int64) (int64, error) {
				assert.Equal(t, "hello", title)
				assert.Equal(t, "body", content)
				assert.Equal(t, []string{"go", "backend"}, tags)
				assert.Equal(t, int64(1), authorID)
				assert.Equal(t, int64(3), boardID)
				return 11, nil
			},
		},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{},
	)

	rr := doJSONRequestWithAuth(t, handler, http.MethodPost, "/boards/3/posts", map[string]any{
		"title":   "hello",
		"content": "body",
		"tags":    []string{"go", "backend"},
	}, 1)

	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.JSONEq(t, `{"id":11}`, rr.Body.String())
}

func TestHandlePublishPost_Success(t *testing.T) {
	handler := newTestHandler(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{
			publishPost: func(id, authorID int64) error {
				assert.Equal(t, int64(5), id)
				assert.Equal(t, int64(1), authorID)
				return nil
			},
		},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{},
	)

	rr := doJSONRequestWithAuth(t, handler, http.MethodPost, "/posts/5/publish", nil, 1)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestHandleCreateComment_WithParentID_Success(t *testing.T) {
	handler := newTestHandler(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{},
		&fakeCommentUseCase{
			createComment: func(content string, authorID, postID int64, parentID *int64) (int64, error) {
				assert.Equal(t, "reply", content)
				assert.Equal(t, int64(1), authorID)
				assert.Equal(t, int64(3), postID)
				require.NotNil(t, parentID)
				assert.Equal(t, int64(9), *parentID)
				return 11, nil
			},
		},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{},
	)

	rr := doJSONRequestWithAuth(t, handler, http.MethodPost, "/posts/3/comments", map[string]any{
		"content":   "reply",
		"parent_id": 9,
	}, 1)

	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.JSONEq(t, `{"id":11}`, rr.Body.String())
}

func TestHandleGetAttachments_Success(t *testing.T) {
	handler := newTestHandler(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{
			getPostAttachments: func(postID int64) ([]model.Attachment, error) {
				assert.Equal(t, int64(3), postID)
				return []model.Attachment{{
					ID:          7,
					PostID:      3,
					FileName:    "a.png",
					ContentType: "image/png",
					SizeBytes:   10,
					StorageKey:  "attachments/a.png",
				}}, nil
			},
		},
	)

	rr := doJSONRequest(t, handler, http.MethodGet, "/posts/3/attachments", nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.JSONEq(t, `{"attachments":[{"id":7,"post_id":3,"file_name":"a.png","content_type":"image/png","size_bytes":10,"file_url":"/api/v1/posts/3/attachments/7/file","preview_url":"/api/v1/posts/3/attachments/7/preview","created_at":"0001-01-01T00:00:00Z"}]}`, rr.Body.String())
}

func TestHandleDeleteAttachment_Success(t *testing.T) {
	handler := newTestHandler(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{
			deletePostAttachment: func(postID, attachmentID, userID int64) error {
				assert.Equal(t, int64(3), postID)
				assert.Equal(t, int64(7), attachmentID)
				assert.Equal(t, int64(1), userID)
				return nil
			},
		},
	)

	rr := doJSONRequestWithAuth(t, handler, http.MethodDelete, "/posts/3/attachments/7", nil, 1)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestHandleUploadAttachment_Success(t *testing.T) {
	handler := newTestHandler(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{
			uploadPostAttachment: func(postID, userID int64, fileName, contentType string, content io.Reader) (*model.AttachmentUpload, error) {
				assert.Equal(t, int64(3), postID)
				assert.Equal(t, int64(1), userID)
				assert.Equal(t, "a.png", fileName)
				assert.Equal(t, "image/png", contentType)
				data, err := io.ReadAll(content)
				require.NoError(t, err)
				assert.Equal(t, "hello", string(data))
				return &model.AttachmentUpload{ID: 8, EmbedMarkdown: "![a.png](attachment://8)"}, nil
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

	req := httptest.NewRequest(http.MethodPost, apiV1Prefix+"/posts/3/attachments/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	tokenProvider := auth.NewJwtTokenProvider("test-secret")
	token, err := tokenProvider.IdToToken(1)
	require.NoError(t, err)
	require.NoError(t, testSessionRepository.Save(1, token, tokenProvider.TTLSeconds()))
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.JSONEq(t, `{"id":8,"embed_markdown":"![a.png](attachment://8)","preview_url":"/api/v1/posts/3/attachments/8/preview"}`, rr.Body.String())
}

func TestHandleUploadAttachment_RejectsOversizedMultipartBeforeUseCase(t *testing.T) {
	called := false
	user := &fakeUserUseCase{}
	tokenProvider := auth.NewJwtTokenProvider("test-secret")
	testSessionRepository = auth.NewCacheSessionRepository(cacheInMemory.NewInMemoryCache())
	sessionUseCase := service.NewSessionService(user, user, tokenProvider, testSessionRepository)
	handler := NewHTTPServer(":0", HTTPDependencies{
		SessionUseCase:  sessionUseCase,
		UserUseCase:     user,
		AccountUseCase:  &fakeAccountUseCase{},
		BoardUseCase:    &fakeBoardUseCase{},
		PostUseCase:     &fakePostUseCase{},
		CommentUseCase:  &fakeCommentUseCase{},
		ReactionUseCase: &fakeReactionUseCase{},
		AttachmentUseCase: &fakeAttachmentUseCase{
			uploadPostAttachment: func(postID, userID int64, fileName, contentType string, content io.Reader) (*model.AttachmentUpload, error) {
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

	req := httptest.NewRequest(http.MethodPost, apiV1Prefix+"/posts/3/attachments/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	token, err := tokenProvider.IdToToken(1)
	require.NoError(t, err)
	require.NoError(t, testSessionRepository.Save(1, token, tokenProvider.TTLSeconds()))
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.False(t, called)
}

func TestHandleGetAttachmentFile_Success(t *testing.T) {
	handler := newTestHandler(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{
			getPostAttachmentFile: func(postID, attachmentID int64) (*model.AttachmentFile, error) {
				assert.Equal(t, int64(3), postID)
				assert.Equal(t, int64(8), attachmentID)
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

	req := httptest.NewRequest(http.MethodGet, apiV1Prefix+"/posts/3/attachments/8/file", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "image/png", rr.Header().Get("Content-Type"))
	assert.Equal(t, "public, max-age=300", rr.Header().Get("Cache-Control"))
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
			getPostAttachmentFile: func(postID, attachmentID int64) (*model.AttachmentFile, error) {
				return &model.AttachmentFile{
					FileName:    "a\"b.png",
					ContentType: "image/png",
					SizeBytes:   5,
					Content:     io.NopCloser(strings.NewReader("hello")),
				}, nil
			},
		},
	)

	req := httptest.NewRequest(http.MethodGet, apiV1Prefix+"/posts/3/attachments/8/file", nil)
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
			getPostAttachmentFile: func(postID, attachmentID int64) (*model.AttachmentFile, error) {
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

	req := httptest.NewRequest(http.MethodGet, apiV1Prefix+"/posts/3/attachments/8/file", nil)
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

	req := httptest.NewRequest(http.MethodGet, apiV1Prefix+"/posts/3/attachments/8/file", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.JSONEq(t, `{"error":"attachment not found"}`, rr.Body.String())
}

func TestHandleGetAttachmentPreview_Success(t *testing.T) {
	handler := newTestHandler(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{
			getPostAttachmentPreviewFile: func(postID, attachmentID, userID int64) (*model.AttachmentFile, error) {
				assert.Equal(t, int64(3), postID)
				assert.Equal(t, int64(8), attachmentID)
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

	req := httptest.NewRequest(http.MethodGet, apiV1Prefix+"/posts/3/attachments/8/preview", nil)
	tokenProvider := auth.NewJwtTokenProvider("test-secret")
	token, err := tokenProvider.IdToToken(1)
	require.NoError(t, err)
	require.NoError(t, testSessionRepository.Save(1, token, tokenProvider.TTLSeconds()))
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
			getPostAttachmentPreviewFile: func(postID, attachmentID, userID int64) (*model.AttachmentFile, error) {
				return &model.AttachmentFile{
					FileName:    "a\"b.png",
					ContentType: "image/png",
					SizeBytes:   5,
					Content:     io.NopCloser(strings.NewReader("hello")),
				}, nil
			},
		},
	)

	req := httptest.NewRequest(http.MethodGet, apiV1Prefix+"/posts/3/attachments/8/preview", nil)
	tokenProvider := auth.NewJwtTokenProvider("test-secret")
	token, err := tokenProvider.IdToToken(1)
	require.NoError(t, err)
	require.NoError(t, testSessionRepository.Save(1, token, tokenProvider.TTLSeconds()))
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	mediaType, params, err := mime.ParseMediaType(rr.Header().Get("Content-Disposition"))
	require.NoError(t, err)
	assert.Equal(t, "inline", mediaType)
	assert.Equal(t, "a\"b.png", params["filename"])
}

func TestHandleGetAttachmentPreview_NotFound(t *testing.T) {
	handler := newTestHandler(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{},
	)

	req := httptest.NewRequest(http.MethodGet, apiV1Prefix+"/posts/3/attachments/8/preview", nil)
	tokenProvider := auth.NewJwtTokenProvider("test-secret")
	token, err := tokenProvider.IdToToken(1)
	require.NoError(t, err)
	require.NoError(t, testSessionRepository.Save(1, token, tokenProvider.TTLSeconds()))
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.JSONEq(t, `{"error":"attachment not found"}`, rr.Body.String())
}

func TestHandleUserSuspend_BadRequestForInvalidDuration(t *testing.T) {
	handler := newTestHandler(
		&fakeUserUseCase{},
		&fakeAccountUseCase{},
		&fakeBoardUseCase{},
		&fakePostUseCase{},
		&fakeCommentUseCase{},
		&fakeReactionUseCase{},
		&fakeAttachmentUseCase{},
	)

	rr := doJSONRequestWithAuth(t, handler, http.MethodPut, "/users/user-uuid-7/suspension", map[string]any{
		"reason":   "spam",
		"duration": "3d",
	}, 1)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleUserUnsuspend_Success(t *testing.T) {
	handler := newTestHandler(
		&fakeUserUseCase{
			unsuspendUser: func(adminID int64, targetUserUUID string) error {
				assert.Equal(t, int64(1), adminID)
				assert.Equal(t, "user-uuid-7", targetUserUUID)
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

	rr := doJSONRequestWithAuth(t, handler, http.MethodDelete, "/users/user-uuid-7/suspension", nil, 1)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestHTTP_UserSignUp_MethodNotAllowed(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/signup", nil)
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHTTP_UserSignUp_Conflict(t *testing.T) {
	user := &fakeUserUseCase{
		signUp: func(username, password string) (string, error) {
			return "", customError.ErrUserAlreadyExists
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
	reaction := &fakeReactionUseCase{
		setReaction: func(userID, targetID int64, targetType entity.ReactionTargetType, reactionType entity.ReactionType) (bool, error) {
			return true, nil
		},
	}
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, reaction, &fakeAttachmentUseCase{})

	rr := doJSONRequestWithAuth(t, handler, http.MethodPut, "/posts/1/reactions/me", map[string]string{
		"reaction_type": "like",
	}, 1)
	assert.Equal(t, http.StatusCreated, rr.Code)
}

func TestHTTP_CommentReactionMeUpdate_NoContent(t *testing.T) {
	reaction := &fakeReactionUseCase{
		setReaction: func(userID, targetID int64, targetType entity.ReactionTargetType, reactionType entity.ReactionType) (bool, error) {
			return false, nil
		},
	}
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, reaction, &fakeAttachmentUseCase{})

	rr := doJSONRequestWithAuth(t, handler, http.MethodPut, "/comments/1/reactions/me", map[string]string{
		"reaction_type": "dislike",
	}, 1)
	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestHTTP_PostReactionMeDelete_NoContent(t *testing.T) {
	reaction := &fakeReactionUseCase{
		deleteReaction: func(userID, targetID int64, targetType entity.ReactionTargetType) error {
			return nil
		},
	}
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, reaction, &fakeAttachmentUseCase{})

	rr := doJSONRequestWithAuth(t, handler, http.MethodDelete, "/posts/1/reactions/me", nil, 1)
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

func TestHTTP_UserDeleteMe_Unauthorized(t *testing.T) {
	account := &fakeAccountUseCase{
		deleteMyAccount: func(userID int64, password string) error {
			return customError.ErrInvalidCredential
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
		createBoard: func(userID int64, name, description string) (int64, error) {
			return 0, customError.ErrForbidden
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
	board := &fakeBoardUseCase{
		deleteBoard: func(id, userID int64) error {
			assert.Equal(t, int64(3), id)
			assert.Equal(t, int64(1), userID)
			return nil
		},
	}
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, board, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequestWithAuth(t, handler, http.MethodDelete, "/boards/3", nil, 1)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestHTTP_BoardPostsGet_Success(t *testing.T) {
	post := &fakePostUseCase{
		getPostsList: func(boardID int64, limit int, lastID int64) (*model.PostList, error) {
			assert.Equal(t, int64(3), boardID)
			assert.Equal(t, 2, limit)
			assert.Equal(t, int64(9), lastID)
			return &model.PostList{
				Posts: []model.Post{{ID: 4, Title: "hello", Content: "world", AuthorUUID: "user-uuid", BoardID: 3}},
				Limit: limit, LastID: lastID, HasMore: false,
			}, nil
		},
	}
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, post, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	req := httptest.NewRequest(http.MethodGet, apiV1Prefix+"/boards/3/posts?limit=2&last_id=9", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"author_uuid":"user-uuid"`)
}

func TestHTTP_PostDetailPut_Success(t *testing.T) {
	post := &fakePostUseCase{
		updatePost: func(id, authorID int64, title, content string, tags []string) error {
			assert.Equal(t, int64(3), id)
			assert.Equal(t, int64(1), authorID)
			assert.Equal(t, "hello", title)
			assert.Equal(t, "world", content)
			assert.Equal(t, []string{"go"}, tags)
			return nil
		},
	}
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, post, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequestWithAuth(t, handler, http.MethodPut, "/posts/3", map[string]any{
		"title":   "hello",
		"content": "world",
		"tags":    []string{"go"},
	}, 1)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestHTTP_PostDetailDelete_Success(t *testing.T) {
	post := &fakePostUseCase{
		deletePost: func(id, authorID int64) error {
			assert.Equal(t, int64(3), id)
			assert.Equal(t, int64(1), authorID)
			return nil
		},
	}
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, post, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequestWithAuth(t, handler, http.MethodDelete, "/posts/3", nil, 1)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestHTTP_PostCommentsGet_Success(t *testing.T) {
	comment := &fakeCommentUseCase{
		getCommentsByPost: func(postID int64, limit int, lastID int64) (*model.CommentList, error) {
			assert.Equal(t, int64(3), postID)
			assert.Equal(t, 2, limit)
			assert.Equal(t, int64(9), lastID)
			return &model.CommentList{
				Comments: []model.Comment{{ID: 4, Content: "nice", AuthorUUID: "user-uuid", PostID: 3}},
				Limit:    limit, LastID: lastID,
			}, nil
		},
	}
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, comment, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	req := httptest.NewRequest(http.MethodGet, apiV1Prefix+"/posts/3/comments?limit=2&last_id=9", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"author_uuid":"user-uuid"`)
}

func TestHTTP_CommentPut_Success(t *testing.T) {
	comment := &fakeCommentUseCase{
		updateComment: func(id, authorID int64, content string) error {
			assert.Equal(t, int64(3), id)
			assert.Equal(t, int64(1), authorID)
			assert.Equal(t, "updated", content)
			return nil
		},
	}
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, comment, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequestWithAuth(t, handler, http.MethodPut, "/comments/3", map[string]any{
		"content": "updated",
	}, 1)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestHTTP_CommentDelete_Success(t *testing.T) {
	comment := &fakeCommentUseCase{
		deleteComment: func(id, authorID int64) error {
			assert.Equal(t, int64(3), id)
			assert.Equal(t, int64(1), authorID)
			return nil
		},
	}
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, comment, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequestWithAuth(t, handler, http.MethodDelete, "/comments/3", nil, 1)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestHTTP_BoardGet_BadLimit(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/boards?limit=bad", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	rr = doJSONRequest(t, handler, http.MethodGet, "/boards?limit=0", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHTTP_BoardGet_BadOffset(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/boards?last_id=bad", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHTTP_BoardWithID_InvalidBoardID(t *testing.T) {
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

func TestHTTP_PostWithID_InvalidPostID(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/posts/abc", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHTTP_PostWithID_NonPositivePostID(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/posts/0", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	rr = doJSONRequest(t, handler, http.MethodGet, "/posts/-1", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHTTP_ReactionDelete_BadUserID(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequest(t, handler, http.MethodDelete, "/posts/1/reactions/me", nil)
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

	rr := doJSONRequest(t, handler, http.MethodGet, "/posts/1/reactions/me", nil)
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHTTP_PostDetail_InternalServerErrorFallback(t *testing.T) {
	post := &fakePostUseCase{
		getPostDetail: func(postID int64) (*model.PostDetail, error) {
			return nil, errors.New("unexpected")
		},
	}
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, post, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/posts/10", nil)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestHTTP_PostDetail_NotFound(t *testing.T) {
	post := &fakePostUseCase{
		getPostDetail: func(postID int64) (*model.PostDetail, error) {
			return nil, customError.ErrPostNotFound
		},
	}
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, post, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/posts/10", nil)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHTTP_PostDetail_IncludesCommentsHasMore(t *testing.T) {
	post := &fakePostUseCase{
		getPostDetail: func(postID int64) (*model.PostDetail, error) {
			return &model.PostDetail{
				Post:            &model.Post{ID: postID, Title: "title"},
				CommentsHasMore: true,
			}, nil
		},
	}
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, post, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/posts/10", nil)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"comments_has_more":true`)
}

func TestHTTP_PostDetail_IncludesTags(t *testing.T) {
	post := &fakePostUseCase{
		getPostDetail: func(postID int64) (*model.PostDetail, error) {
			return &model.PostDetail{
				Post: &model.Post{ID: postID, Title: "hello"},
				Tags: []model.Tag{{ID: 1, Name: "go"}},
			}, nil
		},
	}
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, post, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/posts/10", nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"tags":[{"id":1,"name":"go"`)
}

func TestHTTP_TagPosts_Success(t *testing.T) {
	post := &fakePostUseCase{
		getPostsByTag: func(tagName string, limit int, lastID int64) (*model.PostList, error) {
			assert.Equal(t, "go", tagName)
			assert.Equal(t, 10, limit)
			assert.Equal(t, int64(0), lastID)
			return &model.PostList{
				Posts: []model.Post{{ID: 3, Title: "hello"}},
				Limit: limit,
			}, nil
		},
	}
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, post, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/tags/go/posts?limit=10", nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"posts":[{"id":3`)
}

func TestHTTP_NotFound(t *testing.T) {
	handler := newTestHandler(&fakeUserUseCase{}, &fakeAccountUseCase{}, &fakeBoardUseCase{}, &fakePostUseCase{}, &fakeCommentUseCase{}, &fakeReactionUseCase{}, &fakeAttachmentUseCase{})

	rr := doJSONRequest(t, handler, http.MethodGet, "/unknown", nil)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}
