package entity

import (
	"time"

	"github.com/google/uuid"
)

type NotificationType string

const (
	NotificationTypePostCommented  NotificationType = "post_commented"
	NotificationTypeCommentReplied NotificationType = "comment_replied"
	NotificationTypeMentioned      NotificationType = "mentioned"
)

func ParseNotificationType(raw string) (NotificationType, bool) {
	switch NotificationType(raw) {
	case NotificationTypePostCommented, NotificationTypeCommentReplied, NotificationTypeMentioned:
		return NotificationType(raw), true
	default:
		return "", false
	}
}

type Notification struct {
	ID                     int64
	UUID                   string
	RecipientUserID        int64
	ActorUserID            int64
	Type                   NotificationType
	PostID                 int64
	CommentID              int64
	ActorNameSnapshot      string
	PostTitleSnapshot      string
	CommentPreviewSnapshot string
	ReadAt                 *time.Time
	CreatedAt              time.Time
	DedupKey               string
}

func NewNotification(recipientUserID, actorUserID int64, notificationType NotificationType, postID, commentID int64, actorNameSnapshot, postTitleSnapshot, commentPreviewSnapshot string) *Notification {
	now := time.Now()
	return &Notification{
		UUID:                   uuid.NewString(),
		RecipientUserID:        recipientUserID,
		ActorUserID:            actorUserID,
		Type:                   notificationType,
		PostID:                 postID,
		CommentID:              commentID,
		ActorNameSnapshot:      actorNameSnapshot,
		PostTitleSnapshot:      postTitleSnapshot,
		CommentPreviewSnapshot: commentPreviewSnapshot,
		CreatedAt:              now,
	}
}

func (n *Notification) MarkRead() {
	if n == nil || n.ReadAt != nil {
		return
	}
	now := time.Now()
	n.ReadAt = &now
}
