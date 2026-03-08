package service

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.AttachmentUseCase = (*AttachmentService)(nil)

type AttachmentService struct {
	userRepository       port.UserRepository
	postRepository       port.PostRepository
	attachmentRepository port.AttachmentRepository
	fileStorage          port.FileStorage
	authorizationPolicy  policy.AuthorizationPolicy
}

func NewAttachmentService(userRepository port.UserRepository, postRepository port.PostRepository, attachmentRepository port.AttachmentRepository, fileStorage port.FileStorage, authorizationPolicy policy.AuthorizationPolicy) *AttachmentService {
	return &AttachmentService{
		userRepository:       userRepository,
		postRepository:       postRepository,
		attachmentRepository: attachmentRepository,
		fileStorage:          fileStorage,
		authorizationPolicy:  authorizationPolicy,
	}
}

func (s *AttachmentService) CreatePostAttachment(postID, userID int64, fileName, contentType string, sizeBytes int64, storageKey string) (int64, error) {
	if strings.TrimSpace(fileName) == "" || strings.TrimSpace(contentType) == "" || strings.TrimSpace(storageKey) == "" || sizeBytes <= 0 {
		return 0, customError.ErrInvalidInput
	}
	post, err := s.postRepository.SelectPostByIDIncludingUnpublished(postID)
	if err != nil {
		return 0, customError.WrapRepository("select post by id including unpublished for create attachment", err)
	}
	if post == nil {
		return 0, customError.ErrPostNotFound
	}
	requester, err := s.userRepository.SelectUserByID(userID)
	if err != nil {
		return 0, customError.WrapRepository("select user by id for create attachment", err)
	}
	if requester == nil {
		return 0, customError.ErrUserNotFound
	}
	if err := s.authorizationPolicy.CanWrite(requester); err != nil {
		return 0, err
	}
	if err := s.authorizationPolicy.OwnerOrAdmin(requester, post.AuthorID); err != nil {
		return 0, err
	}
	attachment := entity.NewAttachment(postID, fileName, contentType, sizeBytes, storageKey)
	id, err := s.attachmentRepository.Save(attachment)
	if err != nil {
		return 0, customError.WrapRepository("save attachment", err)
	}
	return id, nil
}

func (s *AttachmentService) UploadPostAttachment(postID, userID int64, fileName, contentType string, content io.Reader) (*model.AttachmentUpload, error) {
	if strings.TrimSpace(fileName) == "" || strings.TrimSpace(contentType) == "" || content == nil {
		return nil, customError.ErrInvalidInput
	}
	data, err := io.ReadAll(content)
	if err != nil {
		return nil, customError.Wrap(customError.ErrInternalServerError, "read upload content", err)
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
	storageKey := buildAttachmentStorageKey(postID, fileName)
	if err := s.fileStorage.Save(storageKey, bytes.NewReader(data)); err != nil {
		return nil, customError.Wrap(customError.ErrInternalServerError, "save upload file", err)
	}
	id, err := s.CreatePostAttachment(postID, userID, fileName, contentType, int64(len(data)), storageKey)
	if err != nil {
		return nil, err
	}
	return &model.AttachmentUpload{
		ID:            id,
		EmbedMarkdown: buildAttachmentEmbedMarkdown(fileName, id),
		PreviewURL:    buildAttachmentPreviewURL(postID, id),
	}, nil
}

func (s *AttachmentService) GetPostAttachments(postID int64) ([]model.Attachment, error) {
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

func (s *AttachmentService) GetPostAttachmentFile(postID, attachmentID int64) (*model.AttachmentFile, error) {
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
	content, err := s.fileStorage.Open(attachment.StorageKey)
	if err != nil {
		return nil, customError.Wrap(customError.ErrInternalServerError, "open attachment file", err)
	}
	return &model.AttachmentFile{
		FileName:    attachment.FileName,
		ContentType: attachment.ContentType,
		SizeBytes:   attachment.SizeBytes,
		Content:     content,
	}, nil
}

func (s *AttachmentService) GetPostAttachmentPreviewFile(postID, attachmentID, userID int64) (*model.AttachmentFile, error) {
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
	content, err := s.fileStorage.Open(attachment.StorageKey)
	if err != nil {
		return nil, customError.Wrap(customError.ErrInternalServerError, "open preview attachment file", err)
	}
	return &model.AttachmentFile{
		FileName:    attachment.FileName,
		ContentType: attachment.ContentType,
		SizeBytes:   attachment.SizeBytes,
		Content:     content,
	}, nil
}

func (s *AttachmentService) DeletePostAttachment(postID, attachmentID, userID int64) error {
	post, err := s.postRepository.SelectPostByIDIncludingUnpublished(postID)
	if err != nil {
		return customError.WrapRepository("select post by id including unpublished for delete attachment", err)
	}
	if post == nil {
		return customError.ErrPostNotFound
	}
	requester, err := s.userRepository.SelectUserByID(userID)
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
	attachment, err := s.attachmentRepository.SelectByID(attachmentID)
	if err != nil {
		return customError.WrapRepository("select attachment by id for delete attachment", err)
	}
	if attachment == nil || attachment.PostID != postID {
		return customError.ErrInvalidInput
	}
	if err := s.fileStorage.Delete(attachment.StorageKey); err != nil {
		return customError.Wrap(customError.ErrInternalServerError, "delete stored file", err)
	}
	if err := s.attachmentRepository.Delete(attachmentID); err != nil {
		return customError.WrapRepository("delete attachment", err)
	}
	return nil
}

func buildAttachmentStorageKey(postID int64, fileName string) string {
	return filepath.ToSlash(filepath.Join("posts", fmt.Sprintf("%d", postID), fileName))
}

func buildAttachmentEmbedMarkdown(fileName string, attachmentID int64) string {
	return fmt.Sprintf("![%s](attachment://%d)", fileName, attachmentID)
}

func buildAttachmentPreviewURL(postID, attachmentID int64) string {
	return fmt.Sprintf("/api/v1/posts/%d/attachments/%d/preview", postID, attachmentID)
}
