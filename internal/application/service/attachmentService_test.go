package service

import (
	"bytes"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"

	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type spyFileStorage struct {
	savedKey     string
	savedContent string
	saveErr      error
	openKey      string
	openContent  string
	openErr      error
	deleteKey    string
	deleteErr    error
}

func (s *spyFileStorage) Save(key string, content io.Reader) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	data, err := io.ReadAll(content)
	if err != nil {
		return err
	}
	s.savedKey = key
	s.savedContent = string(data)
	return nil
}

func (s *spyFileStorage) Delete(key string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.deleteKey = key
	return nil
}

func (s *spyFileStorage) Open(key string) (io.ReadCloser, error) {
	if s.openErr != nil {
		return nil, s.openErr
	}
	s.openKey = key
	return io.NopCloser(strings.NewReader(s.openContent)), nil
}

func testPNGBytes() []byte {
	return []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}
}

func TestAttachmentService_CreatePostAttachment_Success(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	svc := NewAttachmentService(repositories.user, repositories.post, repositories.attachment, &spyFileStorage{}, newTestAuthorizationPolicy())

	id, err := svc.CreatePostAttachment(postID, userID, "a.png", "image/png", 10, "attachments/a.png")
	require.NoError(t, err)
	assert.NotZero(t, id)
}

func TestAttachmentService_GetPostAttachments_RequiresPublishedPost(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	post := seedDraftPost(repositories.post, userID, boardID, "title", "content")
	svc := NewAttachmentService(repositories.user, repositories.post, repositories.attachment, &spyFileStorage{}, newTestAuthorizationPolicy())

	_, err := svc.GetPostAttachments(post)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrPostNotFound))
}

func TestAttachmentService_DeletePostAttachment_ForbiddenForNonOwner(t *testing.T) {
	repositories := newTestRepositories()
	ownerID := seedUser(repositories.user, "alice", "pw", "user")
	otherID := seedUser(repositories.user, "bob", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, ownerID, boardID, "title", "content")
	svc := NewAttachmentService(repositories.user, repositories.post, repositories.attachment, &spyFileStorage{}, newTestAuthorizationPolicy())
	attachmentID, err := svc.CreatePostAttachment(postID, ownerID, "a.png", "image/png", 10, "attachments/a.png")
	require.NoError(t, err)

	err = svc.DeletePostAttachment(postID, attachmentID, otherID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrForbidden))
}

func TestAttachmentService_UploadPostAttachment_SavesFileAndMetadata(t *testing.T) {
	repositories := newTestRepositories()
	storage := &spyFileStorage{}
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedDraftPost(repositories.post, userID, boardID, "title", "content")
	svc := NewAttachmentService(repositories.user, repositories.post, repositories.attachment, storage, newTestAuthorizationPolicy())

	png := testPNGBytes()
	upload, err := svc.UploadPostAttachment(postID, userID, "a.png", "image/png", bytes.NewReader(png))
	require.NoError(t, err)
	require.NotNil(t, upload)
	assert.NotZero(t, upload.ID)
	assert.Equal(t, "![a.png](attachment://"+strconv.FormatInt(upload.ID, 10)+")", upload.EmbedMarkdown)
	assert.Equal(t, "/api/v1/posts/"+strconv.FormatInt(postID, 10)+"/attachments/"+strconv.FormatInt(upload.ID, 10)+"/preview", upload.PreviewURL)
	assert.Contains(t, storage.savedKey, "posts/")
	assert.Contains(t, storage.savedKey, "a.png")
	assert.Equal(t, string(png), storage.savedContent)

	items, err := repositories.attachment.SelectByPostID(postID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, storage.savedKey, items[0].StorageKey)
	assert.Equal(t, int64(len(png)), items[0].SizeBytes)
}

func TestAttachmentService_UploadPostAttachment_RejectsUnsupportedContentType(t *testing.T) {
	repositories := newTestRepositories()
	storage := &spyFileStorage{}
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedDraftPost(repositories.post, userID, boardID, "title", "content")
	svc := NewAttachmentService(repositories.user, repositories.post, repositories.attachment, storage, newTestAuthorizationPolicy())

	_, err := svc.UploadPostAttachment(postID, userID, "a.svg", "image/svg+xml", bytes.NewReader([]byte("<svg></svg>")))
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidInput))
	assert.Empty(t, storage.savedKey)
}

func TestAttachmentService_UploadPostAttachment_RejectsMismatchedSniffedContentType(t *testing.T) {
	repositories := newTestRepositories()
	storage := &spyFileStorage{}
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedDraftPost(repositories.post, userID, boardID, "title", "content")
	svc := NewAttachmentService(repositories.user, repositories.post, repositories.attachment, storage, newTestAuthorizationPolicy())

	_, err := svc.UploadPostAttachment(postID, userID, "a.png", "image/png", strings.NewReader("plain text"))
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidInput))
	assert.Empty(t, storage.savedKey)
}

func TestAttachmentService_UploadPostAttachment_RejectsOversizedFile(t *testing.T) {
	repositories := newTestRepositories()
	storage := &spyFileStorage{}
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedDraftPost(repositories.post, userID, boardID, "title", "content")
	svc := NewAttachmentService(repositories.user, repositories.post, repositories.attachment, storage, newTestAuthorizationPolicy())

	oversized := append(testPNGBytes(), bytes.Repeat([]byte{0}, attachmentMaxSizeBytes)...)
	_, err := svc.UploadPostAttachment(postID, userID, "a.png", "image/png", bytes.NewReader(oversized))
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidInput))
	assert.Empty(t, storage.savedKey)
}

func TestAttachmentService_GetPostAttachmentFile_Success(t *testing.T) {
	repositories := newTestRepositories()
	storage := &spyFileStorage{openContent: "hello"}
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	attachmentID, err := repositories.attachment.Save(entity.NewAttachment(postID, "a.png", "image/png", 5, "posts/1/a.png"))
	require.NoError(t, err)
	svc := NewAttachmentService(repositories.user, repositories.post, repositories.attachment, storage, newTestAuthorizationPolicy())

	file, err := svc.GetPostAttachmentFile(postID, attachmentID)
	require.NoError(t, err)
	require.NotNil(t, file)
	defer file.Content.Close()

	data, err := io.ReadAll(file.Content)
	require.NoError(t, err)
	assert.Equal(t, "posts/1/a.png", storage.openKey)
	assert.Equal(t, "image/png", file.ContentType)
	assert.Equal(t, "a.png", file.FileName)
	assert.NotEmpty(t, file.ETag)
	assert.Equal(t, "hello", string(data))
}

func TestAttachmentService_GetPostAttachmentPreviewFile_AllowedForOwner(t *testing.T) {
	repositories := newTestRepositories()
	storage := &spyFileStorage{openContent: "hello"}
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedDraftPost(repositories.post, userID, boardID, "title", "content")
	attachmentID, err := repositories.attachment.Save(entity.NewAttachment(postID, "a.png", "image/png", 5, "posts/1/a.png"))
	require.NoError(t, err)
	svc := NewAttachmentService(repositories.user, repositories.post, repositories.attachment, storage, newTestAuthorizationPolicy())

	file, err := svc.GetPostAttachmentPreviewFile(postID, attachmentID, userID)
	require.NoError(t, err)
	require.NotNil(t, file)
	defer file.Content.Close()

	data, err := io.ReadAll(file.Content)
	require.NoError(t, err)
	assert.Equal(t, "posts/1/a.png", storage.openKey)
	assert.Equal(t, "hello", string(data))
}
