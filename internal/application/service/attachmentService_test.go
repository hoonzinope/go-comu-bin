package service

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/cache/key"
	"github.com/hoonzinope/go-comu-bin/internal/application/cache/testutil"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type countingReader struct {
	data      []byte
	offset    int
	readBytes int
}

func (r *countingReader) Read(p []byte) (int, error) {
	if r.offset >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.offset:])
	r.offset += n
	r.readBytes += n
	return n, nil
}

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

func (s *spyFileStorage) Save(ctx context.Context, key string, content io.Reader) error {
	_ = ctx
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

func (s *spyFileStorage) Delete(ctx context.Context, key string) error {
	_ = ctx
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.deleteKey = key
	return nil
}

func (s *spyFileStorage) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	_ = ctx
	if s.openErr != nil {
		return nil, s.openErr
	}
	s.openKey = key
	return io.NopCloser(strings.NewReader(s.openContent)), nil
}

type failingAttachmentRepository struct {
	saveErr error
}

func (r *failingAttachmentRepository) Save(context.Context, *entity.Attachment) (int64, error) {
	return 0, r.saveErr
}

func (r *failingAttachmentRepository) SelectByID(context.Context, int64) (*entity.Attachment, error) {
	return nil, nil
}

func (r *failingAttachmentRepository) SelectByPostID(context.Context, int64) ([]*entity.Attachment, error) {
	return nil, nil
}

func (r *failingAttachmentRepository) SelectCleanupCandidatesBefore(context.Context, time.Time, int) ([]*entity.Attachment, error) {
	return nil, nil
}

func (r *failingAttachmentRepository) Update(context.Context, *entity.Attachment) error {
	return nil
}

func (r *failingAttachmentRepository) Delete(context.Context, int64) error {
	return nil
}

type testTxScope struct {
	ctx        context.Context
	user       port.UserRepository
	board      port.BoardRepository
	post       port.PostRepository
	tag        port.TagRepository
	postTag    port.PostTagRepository
	comment    port.CommentRepository
	reaction   port.ReactionRepository
	attachment port.AttachmentRepository
	outbox     port.OutboxAppender
}

func (s testTxScope) Context() context.Context                        { return s.ctx }
func (s testTxScope) UserRepository() port.UserRepository             { return s.user }
func (s testTxScope) BoardRepository() port.BoardRepository           { return s.board }
func (s testTxScope) PostRepository() port.PostRepository             { return s.post }
func (s testTxScope) TagRepository() port.TagRepository               { return s.tag }
func (s testTxScope) PostTagRepository() port.PostTagRepository       { return s.postTag }
func (s testTxScope) CommentRepository() port.CommentRepository       { return s.comment }
func (s testTxScope) ReactionRepository() port.ReactionRepository     { return s.reaction }
func (s testTxScope) AttachmentRepository() port.AttachmentRepository { return s.attachment }
func (s testTxScope) ReportRepository() port.ReportRepository         { return nil }
func (s testTxScope) Outbox() port.OutboxAppender                     { return s.outbox }

type testUnitOfWork struct {
	scope port.TxScope
}

func (u testUnitOfWork) WithinTransaction(ctx context.Context, fn func(tx port.TxScope) error) error {
	if scope, ok := u.scope.(testTxScope); ok {
		scope.ctx = ctx
		return fn(scope)
	}
	return fn(u.scope)
}

func testPNGBytes() []byte {
	return []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}
}

func actualPNGBytes() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 8), G: uint8(y * 8), B: 120, A: 255})
		}
	}
	var out bytes.Buffer
	err := png.Encode(&out, img)
	if err != nil {
		panic(err)
	}
	return out.Bytes()
}

func actualJPEGBytes(quality int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 96, 96))
	for y := 0; y < 96; y++ {
		for x := 0; x < 96; x++ {
			img.Set(x, y, color.RGBA{R: uint8((x*y)%255 + 1), G: uint8((x*3)%255 + 1), B: uint8((y*5)%255 + 1), A: 255})
		}
	}
	var out bytes.Buffer
	err := jpeg.Encode(&out, img, &jpeg.Options{Quality: quality})
	if err != nil {
		panic(err)
	}
	return out.Bytes()
}

func TestAttachmentService_CreatePostAttachment_Success(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	svc := NewAttachmentService(repositories.user, repositories.board, repositories.post, repositories.attachment, repositories.unitOfWork, &spyFileStorage{}, newTestCache(), attachmentDefaultMaxSizeBytes, newTestAuthorizationPolicy())

	id, err := svc.CreatePostAttachment(context.Background(), postID, userID, "a.png", "image/png", 10, "attachments/a.png")
	require.NoError(t, err)
	assert.NotZero(t, id)
}

func TestAttachmentService_CreatePostAttachment_SucceedsWhenCacheInvalidationFails(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	svc := NewAttachmentService(
		repositories.user,
		repositories.board,
		repositories.post,
		repositories.attachment,
		repositories.unitOfWork,
		&spyFileStorage{},
		&errorCache{deleteErr: newCacheFailure(nil)},
		attachmentDefaultMaxSizeBytes,
		newTestAuthorizationPolicy(),
	)

	id, err := svc.CreatePostAttachment(context.Background(), postID, userID, "a.png", "image/png", 10, "attachments/a.png")
	require.NoError(t, err)
	assert.NotZero(t, id)

	items, repoErr := repositories.attachment.SelectByPostID(context.Background(), postID)
	require.NoError(t, repoErr)
	require.Len(t, items, 1)
	assert.Equal(t, id, items[0].ID)
}

func TestAttachmentService_GetPostAttachments_RequiresPublishedPost(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	post := seedDraftPost(repositories.post, userID, boardID, "title", "content")
	svc := NewAttachmentService(repositories.user, repositories.board, repositories.post, repositories.attachment, repositories.unitOfWork, &spyFileStorage{}, newTestCache(), attachmentDefaultMaxSizeBytes, newTestAuthorizationPolicy())

	_, err := svc.GetPostAttachments(context.Background(), post)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrPostNotFound))
}

func TestAttachmentService_GetPostAttachments_ExcludesOrphaned(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	referenced := entity.NewAttachment(postID, "a.png", "image/png", 10, "attachments/a.png")
	referenced.MarkReferenced()
	_, err := repositories.attachment.Save(context.Background(), referenced)
	require.NoError(t, err)
	_, err = repositories.attachment.Save(context.Background(), entity.NewAttachment(postID, "b.png", "image/png", 10, "attachments/b.png"))
	require.NoError(t, err)
	svc := NewAttachmentService(repositories.user, repositories.board, repositories.post, repositories.attachment, repositories.unitOfWork, &spyFileStorage{}, newTestCache(), attachmentDefaultMaxSizeBytes, newTestAuthorizationPolicy())

	items, err := svc.GetPostAttachments(context.Background(), postID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "a.png", items[0].FileName)
}

func TestAttachmentService_HiddenBoard_BlockedForNonAdmin(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "user", "pw", "user")
	authorID := seedUser(repositories.user, "author", "pw", "user")
	boardID := seedBoard(repositories.board, "hidden", "desc")
	postID := seedPost(repositories.post, authorID, boardID, "title", "content")
	board, err := repositories.board.SelectBoardByID(context.Background(), boardID)
	require.NoError(t, err)
	require.NotNil(t, board)
	board.SetHidden(true)
	require.NoError(t, repositories.board.Update(context.Background(), board))
	svc := NewAttachmentService(repositories.user, repositories.board, repositories.post, repositories.attachment, repositories.unitOfWork, &spyFileStorage{}, newTestCache(), attachmentDefaultMaxSizeBytes, newTestAuthorizationPolicy())

	_, err = svc.GetPostAttachments(context.Background(), postID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrBoardNotFound))

	_, err = svc.UploadPostAttachment(context.Background(), postID, userID, "a.png", "image/png", bytes.NewReader(testPNGBytes()))
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrBoardNotFound))
}

func TestAttachmentService_DeletePostAttachment_ForbiddenForNonOwner(t *testing.T) {
	repositories := newTestRepositories()
	ownerID := seedUser(repositories.user, "alice", "pw", "user")
	otherID := seedUser(repositories.user, "bob", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, ownerID, boardID, "title", "content")
	svc := NewAttachmentService(repositories.user, repositories.board, repositories.post, repositories.attachment, repositories.unitOfWork, &spyFileStorage{}, newTestCache(), attachmentDefaultMaxSizeBytes, newTestAuthorizationPolicy())
	attachmentID, err := svc.CreatePostAttachment(context.Background(), postID, ownerID, "a.png", "image/png", 10, "attachments/a.png")
	require.NoError(t, err)

	err = svc.DeletePostAttachment(context.Background(), postID, attachmentID, otherID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrForbidden))
}

func TestAttachmentService_DeletePostAttachment_InvalidatesPostDetailCache(t *testing.T) {
	repositories := newTestRepositories()
	storage := &spyFileStorage{}
	cache := testutil.NewSpyCache()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	svc := NewAttachmentServiceWithActionDispatcher(repositories.user, repositories.board, repositories.post, repositories.attachment, repositories.unitOfWork, storage, cache, newTestActionDispatcher(t, repositories, cache), attachmentDefaultMaxSizeBytes, ImageOptimizationConfig{Enabled: true, JPEGQuality: 82}, newTestAuthorizationPolicy())
	attachmentID, err := svc.CreatePostAttachment(context.Background(), postID, userID, "a.png", "image/png", 10, "attachments/a.png")
	require.NoError(t, err)
	require.NoError(t, cache.Set(context.Background(), key.PostDetail(postID), "stale"))

	err = svc.DeletePostAttachment(context.Background(), postID, attachmentID, userID)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		_, ok, err := cache.Get(context.Background(), key.PostDetail(postID))
		require.NoError(t, err)
		return !ok
	}, time.Second, 10*time.Millisecond)
}

func TestAttachmentService_DeletePostAttachment_RejectsReferencedAttachment(t *testing.T) {
	repositories := newTestRepositories()
	storage := &spyFileStorage{}
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedDraftPost(repositories.post, userID, boardID, "title", "body")
	svc := NewAttachmentService(repositories.user, repositories.board, repositories.post, repositories.attachment, repositories.unitOfWork, storage, newTestCache(), attachmentDefaultMaxSizeBytes, newTestAuthorizationPolicy())
	attachmentID, err := svc.CreatePostAttachment(context.Background(), postID, userID, "a.png", "image/png", 10, "attachments/a.png")
	require.NoError(t, err)

	post, err := repositories.post.SelectPostByIDIncludingUnpublished(context.Background(), postID)
	require.NoError(t, err)
	require.NotNil(t, post)
	post.Update(post.Title, "body ![a](attachment://"+strconv.FormatInt(attachmentID, 10)+")")
	require.NoError(t, repositories.post.Update(context.Background(), post))

	err = svc.DeletePostAttachment(context.Background(), postID, attachmentID, userID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidInput))
	assert.Empty(t, storage.deleteKey)

	stillThere, err := repositories.attachment.SelectByID(context.Background(), attachmentID)
	require.NoError(t, err)
	assert.NotNil(t, stillThere)
}

func TestAttachmentService_DeletePostAttachment_ReturnsAttachmentNotFound_WhenAttachmentMissing(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	svc := NewAttachmentService(repositories.user, repositories.board, repositories.post, repositories.attachment, repositories.unitOfWork, &spyFileStorage{}, newTestCache(), attachmentDefaultMaxSizeBytes, newTestAuthorizationPolicy())

	err := svc.DeletePostAttachment(context.Background(), postID, 999, userID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrAttachmentNotFound))
}

func TestAttachmentService_DeletePostAttachment_MarksPendingDeleteWithoutDeletingStoredFile(t *testing.T) {
	repositories := newTestRepositories()
	storage := &spyFileStorage{deleteErr: errors.New("boom")}
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	svc := NewAttachmentService(repositories.user, repositories.board, repositories.post, repositories.attachment, repositories.unitOfWork, storage, newTestCache(), attachmentDefaultMaxSizeBytes, newTestAuthorizationPolicy())
	attachmentID, err := svc.CreatePostAttachment(context.Background(), postID, userID, "a.png", "image/png", 10, "attachments/a.png")
	require.NoError(t, err)

	err = svc.DeletePostAttachment(context.Background(), postID, attachmentID, userID)
	require.NoError(t, err)

	item, repoErr := repositories.attachment.SelectByID(context.Background(), attachmentID)
	require.NoError(t, repoErr)
	require.NotNil(t, item)
	assert.True(t, item.IsPendingDelete())
	assert.Empty(t, storage.deleteKey)
}

func TestAttachmentService_DeletePostAttachment_HidesPendingDeleteAttachment(t *testing.T) {
	repositories := newTestRepositories()
	storage := &spyFileStorage{openContent: "hello"}
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	svc := NewAttachmentService(repositories.user, repositories.board, repositories.post, repositories.attachment, repositories.unitOfWork, storage, newTestCache(), attachmentDefaultMaxSizeBytes, newTestAuthorizationPolicy())
	attachmentID, err := svc.CreatePostAttachment(context.Background(), postID, userID, "a.png", "image/png", 10, "posts/1/a.png")
	require.NoError(t, err)
	require.NoError(t, svc.DeletePostAttachment(context.Background(), postID, attachmentID, userID))

	items, err := svc.GetPostAttachments(context.Background(), postID)
	require.NoError(t, err)
	assert.Empty(t, items)

	_, err = svc.GetPostAttachmentFile(context.Background(), postID, attachmentID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrAttachmentNotFound))

	_, err = svc.GetPostAttachmentPreviewFile(context.Background(), postID, attachmentID, userID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrAttachmentNotFound))
}

func TestAttachmentService_UploadPostAttachment_SavesFileAndMetadata(t *testing.T) {
	repositories := newTestRepositories()
	storage := &spyFileStorage{}
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedDraftPost(repositories.post, userID, boardID, "title", "content")
	svc := NewAttachmentService(repositories.user, repositories.board, repositories.post, repositories.attachment, repositories.unitOfWork, storage, newTestCache(), attachmentDefaultMaxSizeBytes, newTestAuthorizationPolicy())

	png := testPNGBytes()
	upload, err := svc.UploadPostAttachment(context.Background(), postID, userID, "a.png", "image/png", bytes.NewReader(png))
	require.NoError(t, err)
	require.NotNil(t, upload)
	assert.NotZero(t, upload.ID)
	assert.Equal(t, "![a.png](attachment://"+strconv.FormatInt(upload.ID, 10)+")", upload.EmbedMarkdown)
	assert.Equal(t, "/api/v1/posts/"+strconv.FormatInt(postID, 10)+"/attachments/"+strconv.FormatInt(upload.ID, 10)+"/preview", upload.PreviewURL)
	assert.Contains(t, storage.savedKey, "posts/")
	assert.Contains(t, storage.savedKey, "a.png")
	assert.Equal(t, string(png), storage.savedContent)

	items, err := repositories.attachment.SelectByPostID(context.Background(), postID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, storage.savedKey, items[0].StorageKey)
	assert.Equal(t, int64(len(png)), items[0].SizeBytes)
}

func TestAttachmentService_UploadPostAttachment_EscapesMarkdownInEmbed(t *testing.T) {
	repositories := newTestRepositories()
	storage := &spyFileStorage{}
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedDraftPost(repositories.post, userID, boardID, "title", "content")
	svc := NewAttachmentService(repositories.user, repositories.board, repositories.post, repositories.attachment, repositories.unitOfWork, storage, newTestCache(), attachmentDefaultMaxSizeBytes, newTestAuthorizationPolicy())

	png := testPNGBytes()
	upload, err := svc.UploadPostAttachment(context.Background(), postID, userID, "a](b.png", "image/png", bytes.NewReader(png))
	require.NoError(t, err)
	require.NotNil(t, upload)
	assert.Equal(t, "![a\\]\\(b.png](attachment://"+strconv.FormatInt(upload.ID, 10)+")", upload.EmbedMarkdown)
}

func TestAttachmentService_UploadPostAttachment_InvalidatesPostDetailCache(t *testing.T) {
	repositories := newTestRepositories()
	storage := &spyFileStorage{}
	cache := testutil.NewSpyCache()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedDraftPost(repositories.post, userID, boardID, "title", "content")
	require.NoError(t, cache.Set(context.Background(), key.PostDetail(postID), "stale"))
	svc := NewAttachmentServiceWithActionDispatcher(repositories.user, repositories.board, repositories.post, repositories.attachment, repositories.unitOfWork, storage, cache, newTestActionDispatcher(t, repositories, cache), attachmentDefaultMaxSizeBytes, ImageOptimizationConfig{Enabled: true, JPEGQuality: 82}, newTestAuthorizationPolicy())

	_, err := svc.UploadPostAttachment(context.Background(), postID, userID, "a.png", "image/png", bytes.NewReader(testPNGBytes()))
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		_, ok, err := cache.Get(context.Background(), key.PostDetail(postID))
		require.NoError(t, err)
		return !ok
	}, time.Second, 10*time.Millisecond)
}

func TestAttachmentService_UploadPostAttachment_DeletesStoredFileWhenMetadataSaveFails(t *testing.T) {
	repositories := newTestRepositories()
	storage := &spyFileStorage{}
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedDraftPost(repositories.post, userID, boardID, "title", "content")
	failingUoW := testUnitOfWork{scope: testTxScope{
		user:       repositories.user,
		board:      repositories.board,
		post:       repositories.post,
		tag:        repositories.tag,
		postTag:    repositories.postTag,
		comment:    repositories.comment,
		reaction:   repositories.reaction,
		attachment: &failingAttachmentRepository{saveErr: errors.New("save metadata failed")},
	}}
	svc := NewAttachmentService(
		repositories.user,
		repositories.board,
		repositories.post,
		&failingAttachmentRepository{saveErr: errors.New("save metadata failed")},
		failingUoW,
		storage,
		newTestCache(),
		attachmentDefaultMaxSizeBytes,
		newTestAuthorizationPolicy(),
	)

	_, err := svc.UploadPostAttachment(context.Background(), postID, userID, "a.png", "image/png", bytes.NewReader(testPNGBytes()))
	require.Error(t, err)
	assert.Contains(t, storage.savedKey, "posts/")
	assert.Equal(t, storage.savedKey, storage.deleteKey)
}

func TestAttachmentService_UploadPostAttachment_OptimizesJPEGBeforeSaving(t *testing.T) {
	repositories := newTestRepositories()
	storage := &spyFileStorage{}
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedDraftPost(repositories.post, userID, boardID, "title", "content")
	svc := NewAttachmentServiceWithOptions(
		repositories.user,
		repositories.board,
		repositories.post,
		repositories.attachment,
		repositories.unitOfWork,
		storage,
		newTestCache(),
		attachmentDefaultMaxSizeBytes,
		ImageOptimizationConfig{Enabled: true, JPEGQuality: 60},
		newTestAuthorizationPolicy(),
	)

	original := actualJPEGBytes(100)
	upload, err := svc.UploadPostAttachment(context.Background(), postID, userID, "a.jpg", "image/jpeg", bytes.NewReader(original))
	require.NoError(t, err)
	require.NotNil(t, upload)
	assert.Less(t, len(storage.savedContent), len(original))

	items, err := repositories.attachment.SelectByPostID(context.Background(), postID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, int64(len(storage.savedContent)), items[0].SizeBytes)
}

func TestAttachmentService_UploadPostAttachment_DisabledOptimizationKeepsOriginalBytes(t *testing.T) {
	repositories := newTestRepositories()
	storage := &spyFileStorage{}
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedDraftPost(repositories.post, userID, boardID, "title", "content")
	svc := NewAttachmentServiceWithOptions(
		repositories.user,
		repositories.board,
		repositories.post,
		repositories.attachment,
		repositories.unitOfWork,
		storage,
		newTestCache(),
		attachmentDefaultMaxSizeBytes,
		ImageOptimizationConfig{Enabled: false, JPEGQuality: 60},
		newTestAuthorizationPolicy(),
	)

	original := actualJPEGBytes(100)
	_, err := svc.UploadPostAttachment(context.Background(), postID, userID, "a.jpg", "image/jpeg", bytes.NewReader(original))
	require.NoError(t, err)
	assert.Equal(t, string(original), storage.savedContent)
}

func TestOptimizeAttachmentImage_InvalidPNGFallsBackToOriginal(t *testing.T) {
	original := testPNGBytes()

	optimized := optimizeAttachmentImage("image/png", original, ImageOptimizationConfig{Enabled: true, JPEGQuality: 82})

	assert.Equal(t, original, optimized)
}

func TestAttachmentService_UploadPostAttachment_RejectsUnsupportedContentType(t *testing.T) {
	repositories := newTestRepositories()
	storage := &spyFileStorage{}
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedDraftPost(repositories.post, userID, boardID, "title", "content")
	svc := NewAttachmentService(repositories.user, repositories.board, repositories.post, repositories.attachment, repositories.unitOfWork, storage, newTestCache(), attachmentDefaultMaxSizeBytes, newTestAuthorizationPolicy())

	_, err := svc.UploadPostAttachment(context.Background(), postID, userID, "a.svg", "image/svg+xml", bytes.NewReader([]byte("<svg></svg>")))
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidInput))
	assert.Empty(t, storage.savedKey)
}

func TestAttachmentService_UploadPostAttachment_AcceptsImageJpgAlias(t *testing.T) {
	repositories := newTestRepositories()
	storage := &spyFileStorage{}
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedDraftPost(repositories.post, userID, boardID, "title", "content")
	svc := NewAttachmentService(repositories.user, repositories.board, repositories.post, repositories.attachment, repositories.unitOfWork, storage, newTestCache(), attachmentDefaultMaxSizeBytes, newTestAuthorizationPolicy())

	upload, err := svc.UploadPostAttachment(context.Background(), postID, userID, "a.jpg", "image/jpg", bytes.NewReader([]byte{0xff, 0xd8, 0xff, 0xdb, 0, 0}))
	require.NoError(t, err)
	require.NotNil(t, upload)

	items, err := repositories.attachment.SelectByPostID(context.Background(), postID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "image/jpeg", items[0].ContentType)
}

func TestAttachmentService_UploadPostAttachment_RejectsMismatchedSniffedContentType(t *testing.T) {
	repositories := newTestRepositories()
	storage := &spyFileStorage{}
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedDraftPost(repositories.post, userID, boardID, "title", "content")
	svc := NewAttachmentService(repositories.user, repositories.board, repositories.post, repositories.attachment, repositories.unitOfWork, storage, newTestCache(), attachmentDefaultMaxSizeBytes, newTestAuthorizationPolicy())

	_, err := svc.UploadPostAttachment(context.Background(), postID, userID, "a.png", "image/png", strings.NewReader("plain text"))
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
	svc := NewAttachmentService(repositories.user, repositories.board, repositories.post, repositories.attachment, repositories.unitOfWork, storage, newTestCache(), attachmentDefaultMaxSizeBytes, newTestAuthorizationPolicy())

	oversized := append(testPNGBytes(), bytes.Repeat([]byte{0}, int(attachmentDefaultMaxSizeBytes))...)
	_, err := svc.UploadPostAttachment(context.Background(), postID, userID, "a.png", "image/png", bytes.NewReader(oversized))
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidInput))
	assert.Empty(t, storage.savedKey)
}

func TestAttachmentService_UploadPostAttachment_UsesConfiguredMaxSize(t *testing.T) {
	repositories := newTestRepositories()
	storage := &spyFileStorage{}
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedDraftPost(repositories.post, userID, boardID, "title", "content")
	svc := NewAttachmentService(repositories.user, repositories.board, repositories.post, repositories.attachment, repositories.unitOfWork, storage, newTestCache(), 4, newTestAuthorizationPolicy())

	_, err := svc.UploadPostAttachment(context.Background(), postID, userID, "a.png", "image/png", bytes.NewReader(testPNGBytes()))
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidInput))
}

func TestAttachmentService_UploadPostAttachment_StopsReadingAfterConfiguredLimit(t *testing.T) {
	repositories := newTestRepositories()
	storage := &spyFileStorage{}
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedDraftPost(repositories.post, userID, boardID, "title", "content")
	svc := NewAttachmentService(repositories.user, repositories.board, repositories.post, repositories.attachment, repositories.unitOfWork, storage, newTestCache(), 4, newTestAuthorizationPolicy())

	reader := &countingReader{data: append(testPNGBytes(), bytes.Repeat([]byte{1}, 1024)...)}
	_, err := svc.UploadPostAttachment(context.Background(), postID, userID, "a.png", "image/png", reader)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidInput))
	assert.Equal(t, 5, reader.readBytes)
	assert.Empty(t, storage.savedKey)
}

func TestAttachmentService_UploadPostAttachment_UsesUniqueSanitizedStorageKey(t *testing.T) {
	repositories := newTestRepositories()
	storage := &spyFileStorage{}
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedDraftPost(repositories.post, userID, boardID, "title", "content")
	svc := NewAttachmentService(repositories.user, repositories.board, repositories.post, repositories.attachment, repositories.unitOfWork, storage, newTestCache(), attachmentDefaultMaxSizeBytes, newTestAuthorizationPolicy())

	png := testPNGBytes()
	first, err := svc.UploadPostAttachment(context.Background(), postID, userID, "../my file.png", "image/png", bytes.NewReader(png))
	require.NoError(t, err)
	firstKey := storage.savedKey

	second, err := svc.UploadPostAttachment(context.Background(), postID, userID, "../my file.png", "image/png", bytes.NewReader(png))
	require.NoError(t, err)
	secondKey := storage.savedKey

	require.NotNil(t, first)
	require.NotNil(t, second)
	assert.NotEqual(t, first.ID, second.ID)
	assert.NotEqual(t, firstKey, secondKey)
	assert.True(t, strings.HasPrefix(firstKey, "posts/"+strconv.FormatInt(postID, 10)+"/"))
	assert.True(t, strings.HasSuffix(firstKey, "-my-file.png"))
	assert.True(t, strings.HasSuffix(secondKey, "-my-file.png"))
}

func TestAttachmentService_GetPostAttachmentFile_Success(t *testing.T) {
	repositories := newTestRepositories()
	storage := &spyFileStorage{openContent: "hello"}
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	attachment := entity.NewAttachment(postID, "a.png", "image/png", 5, "posts/1/a.png")
	attachment.MarkReferenced()
	attachmentID, err := repositories.attachment.Save(context.Background(), attachment)
	require.NoError(t, err)
	svc := NewAttachmentService(repositories.user, repositories.board, repositories.post, repositories.attachment, repositories.unitOfWork, storage, newTestCache(), attachmentDefaultMaxSizeBytes, newTestAuthorizationPolicy())

	file, err := svc.GetPostAttachmentFile(context.Background(), postID, attachmentID)
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

func TestAttachmentService_GetPostAttachmentFile_RejectsOrphaned(t *testing.T) {
	repositories := newTestRepositories()
	storage := &spyFileStorage{openContent: "hello"}
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	attachmentID, err := repositories.attachment.Save(context.Background(), entity.NewAttachment(postID, "a.png", "image/png", 5, "posts/1/a.png"))
	require.NoError(t, err)
	svc := NewAttachmentService(repositories.user, repositories.board, repositories.post, repositories.attachment, repositories.unitOfWork, storage, newTestCache(), attachmentDefaultMaxSizeBytes, newTestAuthorizationPolicy())

	_, err = svc.GetPostAttachmentFile(context.Background(), postID, attachmentID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrAttachmentNotFound))
}

func TestAttachmentService_GetPostAttachmentPreviewFile_AllowedForOwner(t *testing.T) {
	repositories := newTestRepositories()
	storage := &spyFileStorage{openContent: "hello"}
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedDraftPost(repositories.post, userID, boardID, "title", "content")
	attachmentID, err := repositories.attachment.Save(context.Background(), entity.NewAttachment(postID, "a.png", "image/png", 5, "posts/1/a.png"))
	require.NoError(t, err)
	svc := NewAttachmentService(repositories.user, repositories.board, repositories.post, repositories.attachment, repositories.unitOfWork, storage, newTestCache(), attachmentDefaultMaxSizeBytes, newTestAuthorizationPolicy())

	file, err := svc.GetPostAttachmentPreviewFile(context.Background(), postID, attachmentID, userID)
	require.NoError(t, err)
	require.NotNil(t, file)
	defer file.Content.Close()

	data, err := io.ReadAll(file.Content)
	require.NoError(t, err)
	assert.Equal(t, "posts/1/a.png", storage.openKey)
	assert.Equal(t, "hello", string(data))
}

func TestAttachmentService_CleanupAttachments_RemovesExpiredOrphans(t *testing.T) {
	repositories := newTestRepositories()
	storage := &spyFileStorage{}
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedDraftPost(repositories.post, userID, boardID, "title", "content")
	expired := entity.NewAttachment(postID, "old.png", "image/png", 5, "posts/1/old.png")
	recent := entity.NewAttachment(postID, "recent.png", "image/png", 5, "posts/1/recent.png")
	referenced := entity.NewAttachment(postID, "live.png", "image/png", 5, "posts/1/live.png")
	referenced.MarkReferenced()
	oldTime := time.Now().Add(-2 * time.Hour)
	recentTime := time.Now().Add(-10 * time.Minute)
	expired.OrphanedAt = &oldTime
	recent.OrphanedAt = &recentTime
	expiredID, err := repositories.attachment.Save(context.Background(), expired)
	require.NoError(t, err)
	recentID, err := repositories.attachment.Save(context.Background(), recent)
	require.NoError(t, err)
	referencedID, err := repositories.attachment.Save(context.Background(), referenced)
	require.NoError(t, err)
	svc := NewAttachmentService(repositories.user, repositories.board, repositories.post, repositories.attachment, repositories.unitOfWork, storage, newTestCache(), attachmentDefaultMaxSizeBytes, newTestAuthorizationPolicy())

	deletedCount, err := svc.CleanupAttachments(context.Background(), time.Now(), time.Hour, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, deletedCount)
	assert.Equal(t, "posts/1/old.png", storage.deleteKey)

	expiredAfter, err := repositories.attachment.SelectByID(context.Background(), expiredID)
	require.NoError(t, err)
	assert.Nil(t, expiredAfter)
	recentAfter, err := repositories.attachment.SelectByID(context.Background(), recentID)
	require.NoError(t, err)
	assert.NotNil(t, recentAfter)
	referencedAfter, err := repositories.attachment.SelectByID(context.Background(), referencedID)
	require.NoError(t, err)
	assert.NotNil(t, referencedAfter)
}

func TestAttachmentService_CleanupAttachments_RemovesPendingDeleteAttachments(t *testing.T) {
	repositories := newTestRepositories()
	storage := &spyFileStorage{}
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedDraftPost(repositories.post, userID, boardID, "title", "content")
	svc := NewAttachmentService(repositories.user, repositories.board, repositories.post, repositories.attachment, repositories.unitOfWork, storage, newTestCache(), attachmentDefaultMaxSizeBytes, newTestAuthorizationPolicy())
	attachmentID, err := svc.CreatePostAttachment(context.Background(), postID, userID, "a.png", "image/png", 10, "posts/1/a.png")
	require.NoError(t, err)
	require.NoError(t, svc.DeletePostAttachment(context.Background(), postID, attachmentID, userID))

	deletedCount, err := svc.CleanupAttachments(context.Background(), time.Now().Add(2*time.Hour), time.Hour, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, deletedCount)
	assert.Equal(t, "posts/1/a.png", storage.deleteKey)

	item, repoErr := repositories.attachment.SelectByID(context.Background(), attachmentID)
	require.NoError(t, repoErr)
	assert.Nil(t, item)
}

func TestAttachmentService_CleanupAttachments_RespectsLimit(t *testing.T) {
	repositories := newTestRepositories()
	storage := &spyFileStorage{}
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedDraftPost(repositories.post, userID, boardID, "title", "content")
	first := entity.NewAttachment(postID, "a.png", "image/png", 5, "posts/1/a.png")
	second := entity.NewAttachment(postID, "b.png", "image/png", 5, "posts/1/b.png")
	oldTime := time.Now().Add(-2 * time.Hour)
	first.OrphanedAt = &oldTime
	second.OrphanedAt = &oldTime
	_, err := repositories.attachment.Save(context.Background(), first)
	require.NoError(t, err)
	_, err = repositories.attachment.Save(context.Background(), second)
	require.NoError(t, err)
	svc := NewAttachmentService(repositories.user, repositories.board, repositories.post, repositories.attachment, repositories.unitOfWork, storage, newTestCache(), attachmentDefaultMaxSizeBytes, newTestAuthorizationPolicy())

	deletedCount, err := svc.CleanupAttachments(context.Background(), time.Now(), time.Hour, 1)
	require.NoError(t, err)
	assert.Equal(t, 1, deletedCount)

	items, err := repositories.attachment.SelectByPostID(context.Background(), postID)
	require.NoError(t, err)
	assert.Len(t, items, 1)
}

func TestAttachmentService_CleanupAttachments_StopsOnStorageDeleteError(t *testing.T) {
	repositories := newTestRepositories()
	storage := &spyFileStorage{deleteErr: errors.New("boom")}
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedDraftPost(repositories.post, userID, boardID, "title", "content")
	attachment := entity.NewAttachment(postID, "a.png", "image/png", 5, "posts/1/a.png")
	oldTime := time.Now().Add(-2 * time.Hour)
	attachment.OrphanedAt = &oldTime
	attachmentID, err := repositories.attachment.Save(context.Background(), attachment)
	require.NoError(t, err)
	svc := NewAttachmentService(repositories.user, repositories.board, repositories.post, repositories.attachment, repositories.unitOfWork, storage, newTestCache(), attachmentDefaultMaxSizeBytes, newTestAuthorizationPolicy())

	deletedCount, err := svc.CleanupAttachments(context.Background(), time.Now(), time.Hour, 10)
	require.Error(t, err)
	assert.Equal(t, 0, deletedCount)

	stillThere, err := repositories.attachment.SelectByID(context.Background(), attachmentID)
	require.NoError(t, err)
	require.NotNil(t, stillThere)
	assert.True(t, stillThere.IsPendingDelete())
}
