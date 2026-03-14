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
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.AttachmentUseCase = (*AttachmentService)(nil)
var _ port.AttachmentCleanupUseCase = (*AttachmentService)(nil)

const attachmentDefaultMaxSizeBytes int64 = 10 << 20

type AttachmentService struct {
	userRepository       port.UserRepository
	boardRepository      port.BoardRepository
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

func NewAttachmentService(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, attachmentRepository port.AttachmentRepository, unitOfWork port.UnitOfWork, fileStorage port.FileStorage, cache port.Cache, maxUploadSizeBytes int64, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *AttachmentService {
	return NewAttachmentServiceWithActionDispatcher(
		userRepository,
		boardRepository,
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

func NewAttachmentServiceWithOptions(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, attachmentRepository port.AttachmentRepository, unitOfWork port.UnitOfWork, fileStorage port.FileStorage, cache port.Cache, maxUploadSizeBytes int64, imageOptimization ImageOptimizationConfig, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *AttachmentService {
	return NewAttachmentServiceWithActionDispatcher(userRepository, boardRepository, postRepository, attachmentRepository, unitOfWork, fileStorage, cache, nil, maxUploadSizeBytes, imageOptimization, authorizationPolicy, logger...)
}

func NewAttachmentServiceWithActionDispatcher(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, attachmentRepository port.AttachmentRepository, unitOfWork port.UnitOfWork, fileStorage port.FileStorage, cache port.Cache, actionDispatcher port.ActionHookDispatcher, maxUploadSizeBytes int64, imageOptimization ImageOptimizationConfig, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *AttachmentService {
	if maxUploadSizeBytes <= 0 {
		maxUploadSizeBytes = attachmentDefaultMaxSizeBytes
	}
	if imageOptimization.JPEGQuality < 1 || imageOptimization.JPEGQuality > 100 {
		imageOptimization.JPEGQuality = 82
	}
	return &AttachmentService{
		userRepository:       userRepository,
		boardRepository:      boardRepository,
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

func (s *AttachmentService) CreatePostAttachment(ctx context.Context, postID, userID int64, fileName, contentType string, sizeBytes int64, storageKey string) (int64, error) {
	if strings.TrimSpace(fileName) == "" || strings.TrimSpace(contentType) == "" || strings.TrimSpace(storageKey) == "" || sizeBytes <= 0 {
		return 0, customerror.ErrInvalidInput
	}
	attachment := entity.NewAttachment(postID, fileName, contentType, sizeBytes, storageKey)
	var id int64
	err := s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		post, err := tx.PostRepository().SelectPostByIDIncludingUnpublished(txCtx, postID)
		if err != nil {
			return customerror.WrapRepository("select post by id including unpublished for create attachment", err)
		}
		if post == nil {
			return customerror.ErrPostNotFound
		}
		requester, err := tx.UserRepository().SelectUserByID(txCtx, userID)
		if err != nil {
			return customerror.WrapRepository("select user by id for create attachment", err)
		}
		if requester == nil {
			return customerror.ErrUserNotFound
		}
		if err := s.ensureBoardVisibleTx(tx, requester, post.BoardID); err != nil {
			return err
		}
		if err := s.authorizationPolicy.CanWrite(requester); err != nil {
			return err
		}
		if err := s.authorizationPolicy.OwnerOrAdmin(requester, post.AuthorID); err != nil {
			return err
		}
		id, err = tx.AttachmentRepository().Save(txCtx, attachment)
		if err != nil {
			return customerror.WrapRepository("save attachment", err)
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
		return nil, customerror.ErrInvalidInput
	}
	data, err := readUploadContent(content, s.maxUploadSizeBytes)
	if err != nil {
		if errors.Is(err, errAttachmentTooLarge) {
			return nil, customerror.ErrInvalidInput
		}
		return nil, customerror.Wrap(customerror.ErrInternalServerError, "read upload content", err)
	}
	if err := validateAttachmentUpload(fileName, contentType, data, s.maxUploadSizeBytes); err != nil {
		return nil, err
	}
	post, err := s.postRepository.SelectPostByIDIncludingUnpublished(ctx, postID)
	if err != nil {
		return nil, customerror.WrapRepository("select post by id including unpublished for upload attachment", err)
	}
	if post == nil {
		return nil, customerror.ErrPostNotFound
	}
	requester, err := s.userRepository.SelectUserByID(ctx, userID)
	if err != nil {
		return nil, customerror.WrapRepository("select user by id for upload attachment", err)
	}
	if requester == nil {
		return nil, customerror.ErrUserNotFound
	}
	if err := s.ensureBoardVisible(ctx, requester, post.BoardID); err != nil {
		return nil, err
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
	if err := s.fileStorage.Save(ctx, storageKey, bytes.NewReader(data)); err != nil {
		return nil, customerror.Wrap(customerror.ErrInternalServerError, "save upload file", err)
	}
	id, err := s.CreatePostAttachment(ctx, postID, userID, fileName, contentType, int64(len(data)), storageKey)
	if err != nil {
		if deleteErr := s.fileStorage.Delete(ctx, storageKey); deleteErr != nil {
			s.logger.Warn(
				"attachment rollback file delete failed",
				"storage_key", storageKey,
				"post_id", postID,
				"user_id", userID,
				"error", deleteErr,
			)
			return nil, errors.Join(err, customerror.Wrap(customerror.ErrInternalServerError, "rollback upload file", deleteErr))
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
	post, err := s.postRepository.SelectPostByID(ctx, postID)
	if err != nil {
		return nil, customerror.WrapRepository("select post by id for get attachments", err)
	}
	if post == nil {
		return nil, customerror.ErrPostNotFound
	}
	if err := s.ensureBoardVisible(ctx, nil, post.BoardID); err != nil {
		return nil, err
	}
	items, err := s.attachmentRepository.SelectByPostID(ctx, postID)
	if err != nil {
		return nil, customerror.WrapRepository("select attachments by post id", err)
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
	post, err := s.postRepository.SelectPostByID(ctx, postID)
	if err != nil {
		return nil, customerror.WrapRepository("select post by id for get attachment file", err)
	}
	if post == nil {
		return nil, customerror.ErrPostNotFound
	}
	if err := s.ensureBoardVisible(ctx, nil, post.BoardID); err != nil {
		return nil, err
	}
	attachment, err := s.attachmentRepository.SelectByID(ctx, attachmentID)
	if err != nil {
		return nil, customerror.WrapRepository("select attachment by id for get attachment file", err)
	}
	if attachment == nil || attachment.PostID != postID {
		return nil, customerror.ErrAttachmentNotFound
	}
	if attachment.IsOrphaned() || attachment.IsPendingDelete() {
		return nil, customerror.ErrAttachmentNotFound
	}
	content, err := s.fileStorage.Open(ctx, attachment.StorageKey)
	if err != nil {
		return nil, customerror.Wrap(customerror.ErrInternalServerError, "open attachment file", err)
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
	post, err := s.postRepository.SelectPostByIDIncludingUnpublished(ctx, postID)
	if err != nil {
		return nil, customerror.WrapRepository("select post by id including unpublished for preview attachment file", err)
	}
	if post == nil {
		return nil, customerror.ErrPostNotFound
	}
	requester, err := s.userRepository.SelectUserByID(ctx, userID)
	if err != nil {
		return nil, customerror.WrapRepository("select user by id for preview attachment file", err)
	}
	if requester == nil {
		return nil, customerror.ErrUserNotFound
	}
	if err := s.ensureBoardVisible(ctx, requester, post.BoardID); err != nil {
		return nil, err
	}
	if err := s.authorizationPolicy.OwnerOrAdmin(requester, post.AuthorID); err != nil {
		return nil, err
	}
	attachment, err := s.attachmentRepository.SelectByID(ctx, attachmentID)
	if err != nil {
		return nil, customerror.WrapRepository("select attachment by id for preview attachment file", err)
	}
	if attachment == nil || attachment.PostID != postID {
		return nil, customerror.ErrAttachmentNotFound
	}
	if attachment.IsPendingDelete() {
		return nil, customerror.ErrAttachmentNotFound
	}
	content, err := s.fileStorage.Open(ctx, attachment.StorageKey)
	if err != nil {
		return nil, customerror.Wrap(customerror.ErrInternalServerError, "open preview attachment file", err)
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
		txCtx := tx.Context()
		post, err := tx.PostRepository().SelectPostByIDIncludingUnpublished(txCtx, postID)
		if err != nil {
			return customerror.WrapRepository("select post by id including unpublished for delete attachment", err)
		}
		if post == nil {
			return customerror.ErrPostNotFound
		}
		requester, err := tx.UserRepository().SelectUserByID(txCtx, userID)
		if err != nil {
			return customerror.WrapRepository("select user by id for delete attachment", err)
		}
		if requester == nil {
			return customerror.ErrUserNotFound
		}
		if err := s.ensureBoardVisibleTx(tx, requester, post.BoardID); err != nil {
			return err
		}
		if err := s.authorizationPolicy.CanWrite(requester); err != nil {
			return err
		}
		if err := s.authorizationPolicy.OwnerOrAdmin(requester, post.AuthorID); err != nil {
			return err
		}
		attachment, err := tx.AttachmentRepository().SelectByID(txCtx, attachmentID)
		if err != nil {
			return customerror.WrapRepository("select attachment by id for delete attachment", err)
		}
		if attachment == nil || attachment.PostID != postID {
			return customerror.ErrAttachmentNotFound
		}
		for _, referencedAttachmentID := range extractAttachmentRefIDs(post.Content) {
			if referencedAttachmentID == attachmentID {
				return customerror.ErrInvalidInput
			}
		}
		updatedAttachment := *attachment
		updatedAttachment.MarkPendingDelete()
		if err := tx.AttachmentRepository().Update(txCtx, &updatedAttachment); err != nil {
			return customerror.WrapRepository("mark attachment pending delete", err)
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
	items, err := s.attachmentRepository.SelectCleanupCandidatesBefore(ctx, cutoff, limit)
	if err != nil {
		return 0, customerror.WrapRepository("select orphan attachments for cleanup", err)
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
			if err := s.attachmentRepository.Update(ctx, item); err != nil {
				return deletedCount, customerror.WrapRepository("mark attachment pending delete for cleanup", err)
			}
		}
		if err := s.fileStorage.Delete(ctx, item.StorageKey); err != nil {
			return deletedCount, customerror.Wrap(customerror.ErrInternalServerError, "delete orphan attachment file", err)
		}
		if err := s.attachmentRepository.Delete(ctx, item.ID); err != nil {
			return deletedCount, customerror.WrapRepository("delete orphan attachment metadata", err)
		}
		deletedCount++
	}
	return deletedCount, nil
}

func (s *AttachmentService) ensureBoardVisible(ctx context.Context, user *entity.User, boardID int64) error {
	board, err := s.boardRepository.SelectBoardByID(ctx, boardID)
	if err != nil {
		return customerror.WrapRepository("select board by id for attachment board visibility", err)
	}
	return policy.EnsureBoardVisible(board, user)
}

func (s *AttachmentService) ensureBoardVisibleTx(tx port.TxScope, user *entity.User, boardID int64) error {
	board, err := tx.BoardRepository().SelectBoardByID(tx.Context(), boardID)
	if err != nil {
		return customerror.WrapRepository("select board by id for attachment board visibility", err)
	}
	return policy.EnsureBoardVisible(board, user)
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
		return customerror.ErrInvalidInput
	}
	if len(data) == 0 || int64(len(data)) > maxUploadSizeBytes {
		return customerror.ErrInvalidInput
	}
	normalizedContentType := normalizeAttachmentContentType(contentType)
	if _, ok := allowedAttachmentContentTypes[normalizedContentType]; !ok {
		return customerror.ErrInvalidInput
	}
	sniffed := http.DetectContentType(data)
	if sniffed == "application/octet-stream" && normalizedContentType == "image/webp" {
		// DetectContentType does not recognize webp reliably for short samples.
		if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
			return nil
		}
	}
	if sniffed != normalizedContentType {
		return customerror.ErrInvalidInput
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
