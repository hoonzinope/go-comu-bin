package attachment

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	svccommon "github.com/hoonzinope/go-comu-bin/internal/application/service/common"
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

const attachmentDefaultMaxSizeBytes int64 = 10 << 20

const DefaultMaxSizeBytes int64 = attachmentDefaultMaxSizeBytes

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
var attachmentEmbedPattern = regexp.MustCompile(`!\[[^\]]*]\(attachment://([0-9a-fA-F-]+)\)`)

type attachmentQueryHandler struct {
	userRepository       port.UserRepository
	boardRepository      port.BoardRepository
	postRepository       port.PostRepository
	attachmentRepository port.AttachmentRepository
	fileStorage          port.FileStorage
	authorizationPolicy  policy.AuthorizationPolicy
}

type QueryHandler = attachmentQueryHandler

func newAttachmentQueryHandler(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, attachmentRepository port.AttachmentRepository, fileStorage port.FileStorage, authorizationPolicy policy.AuthorizationPolicy) *attachmentQueryHandler {
	return &attachmentQueryHandler{userRepository: userRepository, boardRepository: boardRepository, postRepository: postRepository, attachmentRepository: attachmentRepository, fileStorage: fileStorage, authorizationPolicy: authorizationPolicy}
}

func NewQueryHandler(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, attachmentRepository port.AttachmentRepository, fileStorage port.FileStorage, authorizationPolicy policy.AuthorizationPolicy) *QueryHandler {
	return newAttachmentQueryHandler(userRepository, boardRepository, postRepository, attachmentRepository, fileStorage, authorizationPolicy)
}

func (h *attachmentQueryHandler) GetPostAttachments(ctx context.Context, postUUID string) ([]model.Attachment, error) {
	post, err := h.postRepository.SelectPostByUUID(ctx, postUUID)
	if err != nil {
		return nil, customerror.WrapRepository("select post by uuid for get attachments", err)
	}
	if post == nil {
		return nil, customerror.ErrPostNotFound
	}
	if err := policy.EnsureBoardVisibleForUser(ctx, h.boardRepository, nil, post.BoardID, customerror.ErrBoardNotFound, "attachment board visibility"); err != nil {
		return nil, err
	}
	items, err := h.attachmentRepository.SelectByPostID(ctx, post.ID)
	if err != nil {
		return nil, customerror.WrapRepository("select attachments by post id", err)
	}
	out := make([]model.Attachment, 0, len(items))
	for _, item := range items {
		if item.IsOrphaned() || item.IsPendingDelete() {
			continue
		}
		out = append(out, model.Attachment{
			UUID:        item.UUID,
			PostUUID:    post.UUID,
			FileName:    item.FileName,
			ContentType: item.ContentType,
			SizeBytes:   item.SizeBytes,
			StorageKey:  item.StorageKey,
			PreviewURL:  buildAttachmentPreviewURL(post.UUID, item.UUID),
			CreatedAt:   item.CreatedAt,
		})
	}
	return out, nil
}

func (h *attachmentQueryHandler) GetPostAttachmentFile(ctx context.Context, postUUID, attachmentUUID string) (*model.AttachmentFile, error) {
	post, err := h.postRepository.SelectPostByUUID(ctx, postUUID)
	if err != nil {
		return nil, customerror.WrapRepository("select post by uuid for get attachment file", err)
	}
	if post == nil {
		return nil, customerror.ErrPostNotFound
	}
	if err := policy.EnsureBoardVisibleForUser(ctx, h.boardRepository, nil, post.BoardID, customerror.ErrBoardNotFound, "attachment board visibility"); err != nil {
		return nil, err
	}
	attachment, err := h.attachmentRepository.SelectByUUID(ctx, attachmentUUID)
	if err != nil {
		return nil, customerror.WrapRepository("select attachment by uuid for get attachment file", err)
	}
	if attachment == nil || attachment.PostID != post.ID {
		return nil, customerror.ErrAttachmentNotFound
	}
	if attachment.IsOrphaned() || attachment.IsPendingDelete() {
		return nil, customerror.ErrAttachmentNotFound
	}
	content, err := h.fileStorage.Open(ctx, attachment.StorageKey)
	if err != nil {
		return nil, customerror.Wrap(customerror.ErrInternalServerError, "open attachment file", err)
	}
	return &model.AttachmentFile{FileName: attachment.FileName, ContentType: attachment.ContentType, SizeBytes: attachment.SizeBytes, ETag: buildAttachmentETag(attachment), Content: content}, nil
}

func (h *attachmentQueryHandler) GetPostAttachmentPreviewFile(ctx context.Context, postUUID, attachmentUUID string, userID int64) (*model.AttachmentFile, error) {
	post, err := h.postRepository.SelectPostByUUIDIncludingUnpublished(ctx, postUUID)
	if err != nil {
		return nil, customerror.WrapRepository("select post by uuid including unpublished for preview attachment file", err)
	}
	if post == nil {
		return nil, customerror.ErrPostNotFound
	}
	requester, err := h.userRepository.SelectUserByID(ctx, userID)
	if err != nil {
		return nil, customerror.WrapRepository("select user by id for preview attachment file", err)
	}
	if requester == nil {
		return nil, customerror.ErrUserNotFound
	}
	if err := policy.ForbidGuest(requester); err != nil {
		return nil, err
	}
	if err := policy.EnsureBoardVisibleForUser(ctx, h.boardRepository, requester, post.BoardID, customerror.ErrBoardNotFound, "attachment board visibility"); err != nil {
		return nil, err
	}
	if err := h.authorizationPolicy.OwnerOrAdmin(requester, post.AuthorID); err != nil {
		return nil, err
	}
	attachment, err := h.attachmentRepository.SelectByUUID(ctx, attachmentUUID)
	if err != nil {
		return nil, customerror.WrapRepository("select attachment by uuid for preview attachment file", err)
	}
	if attachment == nil || attachment.PostID != post.ID {
		return nil, customerror.ErrAttachmentNotFound
	}
	if attachment.IsPendingDelete() {
		return nil, customerror.ErrAttachmentNotFound
	}
	content, err := h.fileStorage.Open(ctx, attachment.StorageKey)
	if err != nil {
		return nil, customerror.Wrap(customerror.ErrInternalServerError, "open preview attachment file", err)
	}
	return &model.AttachmentFile{FileName: attachment.FileName, ContentType: attachment.ContentType, SizeBytes: attachment.SizeBytes, ETag: buildAttachmentETag(attachment), Content: content}, nil
}

type attachmentCommandHandler struct {
	boardRepository     port.BoardRepository
	postRepository      port.PostRepository
	userRepository      port.UserRepository
	unitOfWork          port.UnitOfWork
	fileStorage         port.FileStorage
	actionDispatcher    port.ActionHookDispatcher
	maxUploadSizeBytes  int64
	imageOptimization   ImageOptimizationConfig
	authorizationPolicy policy.AuthorizationPolicy
	logger              *slog.Logger
}

type CommandHandler = attachmentCommandHandler

func newAttachmentCommandHandler(boardRepository port.BoardRepository, postRepository port.PostRepository, userRepository port.UserRepository, unitOfWork port.UnitOfWork, fileStorage port.FileStorage, actionDispatcher port.ActionHookDispatcher, maxUploadSizeBytes int64, imageOptimization ImageOptimizationConfig, authorizationPolicy policy.AuthorizationPolicy, logger *slog.Logger) *attachmentCommandHandler {
	if maxUploadSizeBytes <= 0 {
		maxUploadSizeBytes = attachmentDefaultMaxSizeBytes
	}
	if imageOptimization.JPEGQuality < 1 || imageOptimization.JPEGQuality > 100 {
		imageOptimization.JPEGQuality = 82
	}
	return &attachmentCommandHandler{boardRepository: boardRepository, postRepository: postRepository, userRepository: userRepository, unitOfWork: unitOfWork, fileStorage: fileStorage, actionDispatcher: actionDispatcher, maxUploadSizeBytes: maxUploadSizeBytes, imageOptimization: imageOptimization, authorizationPolicy: authorizationPolicy, logger: logger}
}

func NewCommandHandler(boardRepository port.BoardRepository, postRepository port.PostRepository, userRepository port.UserRepository, unitOfWork port.UnitOfWork, fileStorage port.FileStorage, actionDispatcher port.ActionHookDispatcher, maxUploadSizeBytes int64, imageOptimization ImageOptimizationConfig, authorizationPolicy policy.AuthorizationPolicy, logger *slog.Logger) *CommandHandler {
	return newAttachmentCommandHandler(boardRepository, postRepository, userRepository, unitOfWork, fileStorage, actionDispatcher, maxUploadSizeBytes, imageOptimization, authorizationPolicy, logger)
}

func (h *attachmentCommandHandler) CreatePostAttachment(ctx context.Context, postUUID string, userID int64, fileName, contentType string, sizeBytes int64, storageKey string) (string, error) {
	if strings.TrimSpace(fileName) == "" || strings.TrimSpace(contentType) == "" || strings.TrimSpace(storageKey) == "" || sizeBytes <= 0 {
		return "", customerror.ErrInvalidInput
	}
	post, err := h.postRepository.SelectPostByUUIDIncludingUnpublished(ctx, postUUID)
	if err != nil {
		return "", customerror.WrapRepository("select post by uuid including unpublished for create attachment", err)
	}
	if post == nil {
		return "", customerror.ErrPostNotFound
	}
	attachment := entity.NewAttachment(post.ID, fileName, contentType, sizeBytes, storageKey)
	var attachmentUUID string
	err = h.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		post, err := tx.PostRepository().SelectPostByUUIDIncludingUnpublished(txCtx, postUUID)
		if err != nil {
			return customerror.WrapRepository("select post by uuid including unpublished for create attachment", err)
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
		if err := policy.ForbidGuest(requester); err != nil {
			return err
		}
		if err := policy.EnsureBoardVisibleForUser(txCtx, tx.BoardRepository(), requester, post.BoardID, customerror.ErrBoardNotFound, "attachment board visibility"); err != nil {
			return err
		}
		if err := h.authorizationPolicy.CanWrite(requester); err != nil {
			return err
		}
		if err := policy.RequireVerifiedEmail(requester); err != nil {
			return err
		}
		if err := h.authorizationPolicy.OwnerOrAdmin(requester, post.AuthorID); err != nil {
			return err
		}
		attachmentID, err := tx.AttachmentRepository().Save(txCtx, attachment)
		if err != nil {
			return customerror.WrapRepository("save attachment", err)
		}
		attachmentUUID = attachment.UUID
		if err := svccommon.DispatchDomainActions(tx, h.actionDispatcher, appevent.NewAttachmentChanged("created", attachmentID, post.ID)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return attachmentUUID, nil
}

func (h *attachmentCommandHandler) UploadPostAttachment(ctx context.Context, postUUID string, userID int64, fileName, contentType string, content io.Reader) (*model.AttachmentUpload, error) {
	if strings.TrimSpace(fileName) == "" || strings.TrimSpace(contentType) == "" || content == nil {
		return nil, customerror.ErrInvalidInput
	}
	post, err := h.postRepository.SelectPostByUUIDIncludingUnpublished(ctx, postUUID)
	if err != nil {
		return nil, customerror.WrapRepository("select post by uuid including unpublished for upload attachment", err)
	}
	if post == nil {
		return nil, customerror.ErrPostNotFound
	}
	requester, err := h.userRepository.SelectUserByID(ctx, userID)
	if err != nil {
		return nil, customerror.WrapRepository("select user by id for upload attachment", err)
	}
	if requester == nil {
		return nil, customerror.ErrUserNotFound
	}
	if err := policy.ForbidGuest(requester); err != nil {
		return nil, err
	}
	if err := policy.EnsureBoardVisibleForUser(ctx, h.boardRepository, requester, post.BoardID, customerror.ErrBoardNotFound, "attachment board visibility"); err != nil {
		return nil, err
	}
	if err := h.authorizationPolicy.CanWrite(requester); err != nil {
		return nil, err
	}
	if err := policy.RequireVerifiedEmail(requester); err != nil {
		return nil, err
	}
	if err := h.authorizationPolicy.OwnerOrAdmin(requester, post.AuthorID); err != nil {
		return nil, err
	}
	data, err := readUploadContent(content, h.maxUploadSizeBytes)
	if err != nil {
		if errors.Is(err, errAttachmentTooLarge) {
			return nil, customerror.ErrInvalidInput
		}
		return nil, customerror.Wrap(customerror.ErrInternalServerError, "read upload content", err)
	}
	if err := validateAttachmentUpload(fileName, contentType, data, h.maxUploadSizeBytes); err != nil {
		return nil, err
	}
	contentType = normalizeAttachmentContentType(contentType)
	data = optimizeAttachmentImage(contentType, data, h.imageOptimization)
	storageKey := buildAttachmentStorageKey(post.ID, fileName)
	if err := h.fileStorage.Save(ctx, storageKey, bytes.NewReader(data)); err != nil {
		return nil, customerror.Wrap(customerror.ErrInternalServerError, "save upload file", err)
	}
	attachmentUUID, err := h.CreatePostAttachment(ctx, postUUID, userID, fileName, contentType, int64(len(data)), storageKey)
	if err != nil {
		if deleteErr := h.fileStorage.Delete(ctx, storageKey); deleteErr != nil {
			h.logger.Warn("attachment rollback file delete failed", "storage_key", storageKey, "post_id", post.ID, "post_uuid", postUUID, "user_id", userID, "error", deleteErr)
			return nil, errors.Join(err, customerror.Wrap(customerror.ErrInternalServerError, "rollback upload file", deleteErr))
		}
		return nil, err
	}
	return &model.AttachmentUpload{UUID: attachmentUUID, EmbedMarkdown: buildAttachmentEmbedMarkdown(fileName, attachmentUUID), PreviewURL: buildAttachmentPreviewURL(postUUID, attachmentUUID)}, nil
}

func (h *attachmentCommandHandler) DeletePostAttachment(ctx context.Context, postUUID, attachmentUUID string, userID int64) error {
	return h.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		post, err := tx.PostRepository().SelectPostByUUIDIncludingUnpublished(txCtx, postUUID)
		if err != nil {
			return customerror.WrapRepository("select post by uuid including unpublished for delete attachment", err)
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
		if err := policy.ForbidGuest(requester); err != nil {
			return err
		}
		if err := policy.EnsureBoardVisibleForUser(txCtx, tx.BoardRepository(), requester, post.BoardID, customerror.ErrBoardNotFound, "attachment board visibility"); err != nil {
			return err
		}
		if err := h.authorizationPolicy.CanWrite(requester); err != nil {
			return err
		}
		if err := policy.RequireVerifiedEmail(requester); err != nil {
			return err
		}
		if err := h.authorizationPolicy.OwnerOrAdmin(requester, post.AuthorID); err != nil {
			return err
		}
		attachment, err := tx.AttachmentRepository().SelectByUUID(txCtx, attachmentUUID)
		if err != nil {
			return customerror.WrapRepository("select attachment by uuid for delete attachment", err)
		}
		if attachment == nil || attachment.PostID != post.ID {
			return customerror.ErrAttachmentNotFound
		}
		for _, referencedAttachmentUUID := range extractAttachmentRefIDs(post.Content) {
			if referencedAttachmentUUID == attachmentUUID {
				return customerror.ErrInvalidInput
			}
		}
		updatedAttachment := *attachment
		updatedAttachment.MarkPendingDelete()
		if err := tx.AttachmentRepository().Update(txCtx, &updatedAttachment); err != nil {
			return customerror.WrapRepository("mark attachment pending delete", err)
		}
		return svccommon.DispatchDomainActions(tx, h.actionDispatcher, appevent.NewAttachmentChanged("deleted", attachment.ID, post.ID))
	})
}

type attachmentCleanupWorkflow struct {
	attachmentRepository port.AttachmentRepository
	fileStorage          port.FileStorage
}

type CleanupWorkflow = attachmentCleanupWorkflow

func newAttachmentCleanupWorkflow(attachmentRepository port.AttachmentRepository, fileStorage port.FileStorage) *attachmentCleanupWorkflow {
	return &attachmentCleanupWorkflow{attachmentRepository: attachmentRepository, fileStorage: fileStorage}
}

func NewCleanupWorkflow(attachmentRepository port.AttachmentRepository, fileStorage port.FileStorage) *CleanupWorkflow {
	return newAttachmentCleanupWorkflow(attachmentRepository, fileStorage)
}

func (w *attachmentCleanupWorkflow) CleanupAttachments(ctx context.Context, now time.Time, gracePeriod time.Duration, limit int) (int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if gracePeriod <= 0 || limit <= 0 {
		return 0, nil
	}
	cutoff := now.Add(-gracePeriod)
	items, err := w.attachmentRepository.SelectCleanupCandidatesBefore(ctx, cutoff, limit)
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
			if err := w.attachmentRepository.Update(ctx, item); err != nil {
				return deletedCount, customerror.WrapRepository("mark attachment pending delete for cleanup", err)
			}
		}
		if err := w.fileStorage.Delete(ctx, item.StorageKey); err != nil {
			return deletedCount, customerror.Wrap(customerror.ErrInternalServerError, "delete orphan attachment file", err)
		}
		if err := w.attachmentRepository.Delete(ctx, item.ID); err != nil {
			return deletedCount, customerror.WrapRepository("delete orphan attachment metadata", err)
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

func buildAttachmentEmbedMarkdown(fileName, attachmentUUID string) string {
	return fmt.Sprintf("![%s](attachment://%s)", markdownSafeAttachmentAltText(fileName), attachmentUUID)
}

func buildAttachmentPreviewURL(postUUID, attachmentUUID string) string {
	return fmt.Sprintf("/api/v1/posts/%s/attachments/%s/preview", postUUID, attachmentUUID)
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

func OptimizeAttachmentImage(contentType string, data []byte, cfg ImageOptimizationConfig) []byte {
	return optimizeAttachmentImage(contentType, data, cfg)
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

func extractAttachmentRefIDs(content string) []string {
	matches := attachmentEmbedPattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		attachmentUUID := strings.TrimSpace(match[1])
		if attachmentUUID == "" {
			continue
		}
		if _, exists := seen[attachmentUUID]; exists {
			continue
		}
		seen[attachmentUUID] = struct{}{}
		out = append(out, attachmentUUID)
	}
	return out
}
