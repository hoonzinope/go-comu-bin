package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	appevent "github.com/hoonzinope/go-comu-bin/internal/application/event"
	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.AttachmentUseCase = (*AttachmentService)(nil)
var _ port.AttachmentCleanupUseCase = (*AttachmentService)(nil)

const attachmentDefaultMaxSizeBytes int64 = 10 << 20

type AttachmentService struct {
	userRepository       port.UserRepository
	postRepository       port.PostRepository
	attachmentRepository port.AttachmentRepository
	unitOfWork           port.UnitOfWork
	fileStorage          port.FileStorage
	cache                port.Cache
	actionDispatcher     port.ActionHookDispatcher
	maxUploadSizeBytes   int64
	imageOptimization    ImageOptimizationConfig
	authorizationPolicy  policy.AuthorizationPolicy
	logger               *slog.Logger
}

type ImageOptimizationConfig struct {
	Enabled     bool
	JPEGQuality int
}

var allowedAttachmentContentTypes = map[string]struct{}{
	"image/png":  {},
	"image/jpeg": {},
	"image/gif":  {},
	"image/webp": {},
}

var attachmentFileNameSanitizer = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func NewAttachmentService(userRepository port.UserRepository, postRepository port.PostRepository, attachmentRepository port.AttachmentRepository, unitOfWork port.UnitOfWork, fileStorage port.FileStorage, cache port.Cache, maxUploadSizeBytes int64, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *AttachmentService {
	return NewAttachmentServiceWithActionDispatcher(
		userRepository,
		postRepository,
		attachmentRepository,
		unitOfWork,
		fileStorage,
		cache,
		nil,
		maxUploadSizeBytes,
		ImageOptimizationConfig{Enabled: true, JPEGQuality: 82},
		authorizationPolicy,
		resolveLogger(logger),
	)
}

func NewAttachmentServiceWithOptions(userRepository port.UserRepository, postRepository port.PostRepository, attachmentRepository port.AttachmentRepository, unitOfWork port.UnitOfWork, fileStorage port.FileStorage, cache port.Cache, maxUploadSizeBytes int64, imageOptimization ImageOptimizationConfig, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *AttachmentService {
	return NewAttachmentServiceWithActionDispatcher(userRepository, postRepository, attachmentRepository, unitOfWork, fileStorage, cache, nil, maxUploadSizeBytes, imageOptimization, authorizationPolicy, logger...)
}

func NewAttachmentServiceWithActionDispatcher(userRepository port.UserRepository, postRepository port.PostRepository, attachmentRepository port.AttachmentRepository, unitOfWork port.UnitOfWork, fileStorage port.FileStorage, cache port.Cache, actionDispatcher port.ActionHookDispatcher, maxUploadSizeBytes int64, imageOptimization ImageOptimizationConfig, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *AttachmentService {
	if maxUploadSizeBytes <= 0 {
		maxUploadSizeBytes = attachmentDefaultMaxSizeBytes
	}
	if imageOptimization.JPEGQuality < 1 || imageOptimization.JPEGQuality > 100 {
		imageOptimization.JPEGQuality = 82
	}
	return &AttachmentService{
		userRepository:       userRepository,
		postRepository:       postRepository,
		attachmentRepository: attachmentRepository,
		unitOfWork:           unitOfWork,
		fileStorage:          fileStorage,
		cache:                cache,
		actionDispatcher:     resolveActionDispatcher(actionDispatcher),
		maxUploadSizeBytes:   maxUploadSizeBytes,
		imageOptimization:    imageOptimization,
		authorizationPolicy:  authorizationPolicy,
		logger:               resolveLogger(logger),
	}
}

// Deprecated: use NewAttachmentServiceWithActionDispatcher.
func NewAttachmentServiceWithPublisher(userRepository port.UserRepository, postRepository port.PostRepository, attachmentRepository port.AttachmentRepository, unitOfWork port.UnitOfWork, fileStorage port.FileStorage, cache port.Cache, publisher port.EventPublisher, maxUploadSizeBytes int64, imageOptimization ImageOptimizationConfig, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *AttachmentService {
	return NewAttachmentServiceWithActionDispatcher(userRepository, postRepository, attachmentRepository, unitOfWork, fileStorage, cache, wrapEventPublisherAsActionDispatcher(publisher), maxUploadSizeBytes, imageOptimization, authorizationPolicy, logger...)
}

func (s *AttachmentService) CreatePostAttachment(ctx context.Context, postID, userID int64, fileName, contentType string, sizeBytes int64, storageKey string) (int64, error) {
	if strings.TrimSpace(fileName) == "" || strings.TrimSpace(contentType) == "" || strings.TrimSpace(storageKey) == "" || sizeBytes <= 0 {
		return 0, customError.ErrInvalidInput
	}
	attachment := entity.NewAttachment(postID, fileName, contentType, sizeBytes, storageKey)
	var id int64
	err := s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		post, err := tx.PostRepository().SelectPostByIDIncludingUnpublished(postID)
		if err != nil {
			return customError.WrapRepository("select post by id including unpublished for create attachment", err)
		}
		if post == nil {
			return customError.ErrPostNotFound
		}
		requester, err := tx.UserRepository().SelectUserByID(userID)
		if err != nil {
			return customError.WrapRepository("select user by id for create attachment", err)
		}
		if requester == nil {
			return customError.ErrUserNotFound
		}
		if err := s.authorizationPolicy.CanWrite(requester); err != nil {
			return err
		}
		if err := s.authorizationPolicy.OwnerOrAdmin(requester, post.AuthorID); err != nil {
			return err
		}
		id, err = tx.AttachmentRepository().Save(attachment)
		if err != nil {
			return customError.WrapRepository("save attachment", err)
		}
		if err := dispatchDomainActions(tx, s.actionDispatcher, appevent.NewAttachmentChanged("created", id, postID)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (s *AttachmentService) UploadPostAttachment(ctx context.Context, postID, userID int64, fileName, contentType string, content io.Reader) (*model.AttachmentUpload, error) {
	if strings.TrimSpace(fileName) == "" || strings.TrimSpace(contentType) == "" || content == nil {
		return nil, customError.ErrInvalidInput
	}
	data, err := readUploadContent(content, s.maxUploadSizeBytes)
	if err != nil {
		if errors.Is(err, errAttachmentTooLarge) {
			return nil, customError.ErrInvalidInput
		}
		return nil, customError.Wrap(customError.ErrInternalServerError, "read upload content", err)
	}
	if err := validateAttachmentUpload(fileName, contentType, data, s.maxUploadSizeBytes); err != nil {
		return nil, err
	}
	post, err := s.postRepository.SelectPostByIDIncludingUnpublished(postID)
	if err != nil {
		return nil, customError.WrapRepository("select post by id including unpublished for upload attachment", err)
	}
	if post == nil {
		return nil, customError.ErrPostNotFound
	}
	requester, err := s.userRepository.SelectUserByID(userID)
	if err != nil {
		return nil, customError.WrapRepository("select user by id for upload attachment", err)
	}
	if requester == nil {
		return nil, customError.ErrUserNotFound
	}
	if err := s.authorizationPolicy.CanWrite(requester); err != nil {
		return nil, err
	}
	if err := s.authorizationPolicy.OwnerOrAdmin(requester, post.AuthorID); err != nil {
		return nil, err
	}
	contentType = normalizeAttachmentContentType(contentType)
	data = optimizeAttachmentImage(contentType, data, s.imageOptimization)
	storageKey := buildAttachmentStorageKey(postID, fileName)
	if err := s.fileStorage.Save(storageKey, bytes.NewReader(data)); err != nil {
		return nil, customError.Wrap(customError.ErrInternalServerError, "save upload file", err)
	}
	id, err := s.CreatePostAttachment(ctx, postID, userID, fileName, contentType, int64(len(data)), storageKey)
	if err != nil {
		if deleteErr := s.fileStorage.Delete(storageKey); deleteErr != nil {
			return nil, errors.Join(err, customError.Wrap(customError.ErrInternalServerError, "rollback upload file", deleteErr))
		}
		return nil, err
	}
	return &model.AttachmentUpload{
		ID:            id,
		EmbedMarkdown: buildAttachmentEmbedMarkdown(fileName, id),
		PreviewURL:    buildAttachmentPreviewURL(postID, id),
	}, nil
}

func (s *AttachmentService) GetPostAttachments(ctx context.Context, postID int64) ([]model.Attachment, error) {
	post, err := s.postRepository.SelectPostByID(postID)
	if err != nil {
		return nil, customError.WrapRepository("select post by id for get attachments", err)
	}
	if post == nil {
		return nil, customError.ErrPostNotFound
	}
	items, err := s.attachmentRepository.SelectByPostID(postID)
	if err != nil {
		return nil, customError.WrapRepository("select attachments by post id", err)
	}
	out := make([]model.Attachment, 0, len(items))
	for _, item := range items {
		if item.IsOrphaned() || item.IsPendingDelete() {
			continue
		}
		out = append(out, model.Attachment{
			ID:          item.ID,
			PostID:      item.PostID,
			FileName:    item.FileName,
			ContentType: item.ContentType,
			SizeBytes:   item.SizeBytes,
			StorageKey:  item.StorageKey,
			PreviewURL:  buildAttachmentPreviewURL(item.PostID, item.ID),
			CreatedAt:   item.CreatedAt,
		})
	}
	return out, nil
}

func (s *AttachmentService) GetPostAttachmentFile(ctx context.Context, postID, attachmentID int64) (*model.AttachmentFile, error) {
	post, err := s.postRepository.SelectPostByID(postID)
	if err != nil {
		return nil, customError.WrapRepository("select post by id for get attachment file", err)
	}
	if post == nil {
		return nil, customError.ErrPostNotFound
	}
	attachment, err := s.attachmentRepository.SelectByID(attachmentID)
	if err != nil {
		return nil, customError.WrapRepository("select attachment by id for get attachment file", err)
	}
	if attachment == nil || attachment.PostID != postID {
		return nil, customError.ErrAttachmentNotFound
	}
	if attachment.IsOrphaned() || attachment.IsPendingDelete() {
		return nil, customError.ErrAttachmentNotFound
	}
	content, err := s.fileStorage.Open(attachment.StorageKey)
	if err != nil {
		return nil, customError.Wrap(customError.ErrInternalServerError, "open attachment file", err)
	}
	return &model.AttachmentFile{
		FileName:    attachment.FileName,
		ContentType: attachment.ContentType,
		SizeBytes:   attachment.SizeBytes,
		ETag:        buildAttachmentETag(attachment),
		Content:     content,
	}, nil
}

func (s *AttachmentService) GetPostAttachmentPreviewFile(ctx context.Context, postID, attachmentID, userID int64) (*model.AttachmentFile, error) {
	post, err := s.postRepository.SelectPostByIDIncludingUnpublished(postID)
	if err != nil {
		return nil, customError.WrapRepository("select post by id including unpublished for preview attachment file", err)
	}
	if post == nil {
		return nil, customError.ErrPostNotFound
	}
	requester, err := s.userRepository.SelectUserByID(userID)
	if err != nil {
		return nil, customError.WrapRepository("select user by id for preview attachment file", err)
	}
	if requester == nil {
		return nil, customError.ErrUserNotFound
	}
	if err := s.authorizationPolicy.OwnerOrAdmin(requester, post.AuthorID); err != nil {
		return nil, err
	}
	attachment, err := s.attachmentRepository.SelectByID(attachmentID)
	if err != nil {
		return nil, customError.WrapRepository("select attachment by id for preview attachment file", err)
	}
	if attachment == nil || attachment.PostID != postID {
		return nil, customError.ErrAttachmentNotFound
	}
	if attachment.IsPendingDelete() {
		return nil, customError.ErrAttachmentNotFound
	}
	content, err := s.fileStorage.Open(attachment.StorageKey)
	if err != nil {
		return nil, customError.Wrap(customError.ErrInternalServerError, "open preview attachment file", err)
	}
	return &model.AttachmentFile{
		FileName:    attachment.FileName,
		ContentType: attachment.ContentType,
		SizeBytes:   attachment.SizeBytes,
		ETag:        buildAttachmentETag(attachment),
		Content:     content,
	}, nil
}

func (s *AttachmentService) DeletePostAttachment(ctx context.Context, postID, attachmentID, userID int64) error {
	err := s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		post, err := tx.PostRepository().SelectPostByIDIncludingUnpublished(postID)
		if err != nil {
			return customError.WrapRepository("select post by id including unpublished for delete attachment", err)
		}
		if post == nil {
			return customError.ErrPostNotFound
		}
		requester, err := tx.UserRepository().SelectUserByID(userID)
		if err != nil {
			return customError.WrapRepository("select user by id for delete attachment", err)
		}
		if requester == nil {
			return customError.ErrUserNotFound
		}
		if err := s.authorizationPolicy.CanWrite(requester); err != nil {
			return err
		}
		if err := s.authorizationPolicy.OwnerOrAdmin(requester, post.AuthorID); err != nil {
			return err
		}
		attachment, err := tx.AttachmentRepository().SelectByID(attachmentID)
		if err != nil {
			return customError.WrapRepository("select attachment by id for delete attachment", err)
		}
		if attachment == nil || attachment.PostID != postID {
			return customError.ErrAttachmentNotFound
		}
		for _, referencedAttachmentID := range extractAttachmentRefIDs(post.Content) {
			if referencedAttachmentID == attachmentID {
				return customError.ErrInvalidInput
			}
		}
		updatedAttachment := *attachment
		updatedAttachment.MarkPendingDelete()
		if err := tx.AttachmentRepository().Update(&updatedAttachment); err != nil {
			return customError.WrapRepository("mark attachment pending delete", err)
		}
		if err := dispatchDomainActions(tx, s.actionDispatcher, appevent.NewAttachmentChanged("deleted", attachmentID, postID)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *AttachmentService) CleanupAttachments(ctx context.Context, now time.Time, gracePeriod time.Duration, limit int) (int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if gracePeriod <= 0 || limit <= 0 {
		return 0, nil
	}
	cutoff := now.Add(-gracePeriod)
	items, err := s.attachmentRepository.SelectCleanupCandidatesBefore(cutoff, limit)
	if err != nil {
		return 0, customError.WrapRepository("select orphan attachments for cleanup", err)
	}
	deletedCount := 0
	for _, item := range items {
		select {
		case <-ctx.Done():
			return deletedCount, ctx.Err()
		default:
		}
		if !item.IsPendingDelete() {
			item.MarkPendingDeleteAt(cutoff)
			if err := s.attachmentRepository.Update(item); err != nil {
				return deletedCount, customError.WrapRepository("mark attachment pending delete for cleanup", err)
			}
		}
		if err := s.fileStorage.Delete(item.StorageKey); err != nil {
			return deletedCount, customError.Wrap(customError.ErrInternalServerError, "delete orphan attachment file", err)
		}
		if err := s.attachmentRepository.Delete(item.ID); err != nil {
			return deletedCount, customError.WrapRepository("delete orphan attachment metadata", err)
		}
		deletedCount++
	}
	return deletedCount, nil
}

func buildAttachmentStorageKey(postID int64, fileName string) string {
	suffix, err := newAttachmentKeySuffix()
	if err != nil {
		suffix = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return filepath.ToSlash(filepath.Join("posts", fmt.Sprintf("%d", postID), suffix+"-"+sanitizeAttachmentFileName(fileName)))
}

func buildAttachmentEmbedMarkdown(fileName string, attachmentID int64) string {
	return fmt.Sprintf("![%s](attachment://%d)", markdownSafeAttachmentAltText(fileName), attachmentID)
}

func buildAttachmentPreviewURL(postID, attachmentID int64) string {
	return fmt.Sprintf("/api/v1/posts/%d/attachments/%d/preview", postID, attachmentID)
}

func buildAttachmentETag(attachment *entity.Attachment) string {
	return fmt.Sprintf("\"att-%d-%d-%d\"", attachment.ID, attachment.SizeBytes, attachment.CreatedAt.Unix())
}

func validateAttachmentUpload(fileName, contentType string, data []byte, maxUploadSizeBytes int64) error {
	if strings.TrimSpace(fileName) == "" || strings.TrimSpace(contentType) == "" {
		return customError.ErrInvalidInput
	}
	if len(data) == 0 || int64(len(data)) > maxUploadSizeBytes {
		return customError.ErrInvalidInput
	}
	normalizedContentType := normalizeAttachmentContentType(contentType)
	if _, ok := allowedAttachmentContentTypes[normalizedContentType]; !ok {
		return customError.ErrInvalidInput
	}
	sniffed := http.DetectContentType(data)
	if sniffed == "application/octet-stream" && normalizedContentType == "image/webp" {
		// DetectContentType does not recognize webp reliably for short samples.
		if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
			return nil
		}
	}
	if sniffed != normalizedContentType {
		return customError.ErrInvalidInput
	}
	return nil
}

func normalizeAttachmentContentType(contentType string) string {
	switch strings.TrimSpace(contentType) {
	case "image/jpg":
		return "image/jpeg"
	default:
		return strings.TrimSpace(contentType)
	}
}

func optimizeAttachmentImage(contentType string, data []byte, cfg ImageOptimizationConfig) []byte {
	if !cfg.Enabled {
		return data
	}
	switch contentType {
	case "image/jpeg":
		return optimizeJPEG(data, cfg.JPEGQuality)
	case "image/png":
		return optimizePNG(data)
	default:
		return data
	}
}

func optimizeJPEG(data []byte, quality int) []byte {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return data
	}
	var out bytes.Buffer
	if err := jpeg.Encode(&out, img, &jpeg.Options{Quality: quality}); err != nil {
		return data
	}
	if out.Len() == 0 || out.Len() >= len(data) {
		return data
	}
	return out.Bytes()
}

func optimizePNG(data []byte) []byte {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return data
	}
	var out bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestCompression}
	if err := encoder.Encode(&out, img); err != nil {
		return data
	}
	if out.Len() == 0 || out.Len() >= len(data) {
		return data
	}
	return out.Bytes()
}

func sanitizeAttachmentFileName(fileName string) string {
	base := filepath.Base(strings.TrimSpace(fileName))
	sanitized := attachmentFileNameSanitizer.ReplaceAllString(base, "-")
	sanitized = strings.Trim(sanitized, "-.")
	if sanitized == "" {
		return "file"
	}
	return sanitized
}

func markdownSafeAttachmentAltText(fileName string) string {
	alt := strings.TrimSpace(fileName)
	alt = strings.NewReplacer(
		`\`, `\\`,
		`[`, `\[`,
		`]`, `\]`,
		`(`, `\(`,
		`)`, `\)`,
		"\r", " ",
		"\n", " ",
	).Replace(alt)
	if alt == "" {
		return "attachment"
	}
	return alt
}

func newAttachmentKeySuffix() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

var errAttachmentTooLarge = errors.New("attachment too large")

func readUploadContent(content io.Reader, maxUploadSizeBytes int64) ([]byte, error) {
	if maxUploadSizeBytes <= 0 {
		return io.ReadAll(content)
	}
	limited := &io.LimitedReader{R: content, N: maxUploadSizeBytes + 1}
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxUploadSizeBytes {
		return nil, errAttachmentTooLarge
	}
	return data, nil
}
