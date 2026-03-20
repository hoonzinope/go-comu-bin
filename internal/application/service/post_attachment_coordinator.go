package service

import (
	"context"
	"regexp"
	"strings"

	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var attachmentEmbedPattern = regexp.MustCompile(`!\[[^\]]*]\(attachment://([0-9a-fA-F-]+)\)`)

type postAttachmentCoordinator struct {
	attachmentRepository port.AttachmentRepository
}

func newPostAttachmentCoordinator(attachmentRepository port.AttachmentRepository) *postAttachmentCoordinator {
	return &postAttachmentCoordinator{attachmentRepository: attachmentRepository}
}

func (c *postAttachmentCoordinator) validateAttachmentRefs(ctx context.Context, postID int64, content string) error {
	return c.validateAttachmentRefsWithRepo(ctx, c.attachmentRepository, postID, content)
}

func (c *postAttachmentCoordinator) validateAttachmentRefsWithRepo(ctx context.Context, repo port.AttachmentRepository, postID int64, content string) error {
	for _, attachmentUUID := range extractAttachmentRefIDs(content) {
		attachment, err := repo.SelectByUUID(ctx, attachmentUUID)
		if err != nil {
			return customerror.WrapRepository("select attachment by uuid for validate post attachments", err)
		}
		if attachment == nil || attachment.PostID != postID || attachment.IsPendingDelete() {
			return customerror.ErrInvalidInput
		}
	}
	return nil
}

func (c *postAttachmentCoordinator) syncPostAttachmentOrphans(ctx context.Context, repo port.AttachmentRepository, postID int64, content string) error {
	items, err := repo.SelectByPostID(ctx, postID)
	if err != nil {
		return customerror.WrapRepository("select attachments by post id for sync orphans", err)
	}
	refIDs := make(map[string]struct{})
	for _, attachmentUUID := range extractAttachmentRefIDs(content) {
		refIDs[attachmentUUID] = struct{}{}
	}
	for _, item := range items {
		if _, ok := refIDs[item.UUID]; ok {
			item.MarkReferenced()
		} else {
			item.MarkOrphaned()
		}
		if err := repo.Update(ctx, item); err != nil {
			return customerror.WrapRepository("update attachment orphan state", err)
		}
	}
	return nil
}

func (c *postAttachmentCoordinator) orphanPostAttachments(ctx context.Context, repo port.AttachmentRepository, postID int64) error {
	items, err := repo.SelectByPostID(ctx, postID)
	if err != nil {
		return customerror.WrapRepository("select attachments by post id for delete post", err)
	}
	for _, item := range items {
		item.MarkOrphaned()
		if err := repo.Update(ctx, item); err != nil {
			return customerror.WrapRepository("orphan attachments for delete post", err)
		}
	}
	return nil
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

func attachmentsFromEntities(postUUID string, items []*entity.Attachment) []model.Attachment {
	out := make([]model.Attachment, 0, len(items))
	for _, item := range items {
		if item.IsOrphaned() || item.IsPendingDelete() {
			continue
		}
		out = append(out, model.Attachment{
			UUID:        item.UUID,
			PostUUID:    postUUID,
			FileName:    item.FileName,
			ContentType: item.ContentType,
			SizeBytes:   item.SizeBytes,
			StorageKey:  item.StorageKey,
			CreatedAt:   item.CreatedAt,
		})
	}
	return out
}
