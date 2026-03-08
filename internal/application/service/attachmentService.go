package service

import (
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
	authorizationPolicy  policy.AuthorizationPolicy
}

func NewAttachmentService(userRepository port.UserRepository, postRepository port.PostRepository, attachmentRepository port.AttachmentRepository, authorizationPolicy policy.AuthorizationPolicy) *AttachmentService {
	return &AttachmentService{
		userRepository:       userRepository,
		postRepository:       postRepository,
		attachmentRepository: attachmentRepository,
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
			CreatedAt:   item.CreatedAt,
		})
	}
	return out, nil
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
	if err := s.attachmentRepository.Delete(attachmentID); err != nil {
		return customError.WrapRepository("delete attachment", err)
	}
	return nil
}
