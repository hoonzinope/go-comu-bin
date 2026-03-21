package model

import "time"

type NotificationType string

const (
	NotificationTypePostCommented  NotificationType = "post_commented"
	NotificationTypeCommentReplied NotificationType = "comment_replied"
	NotificationTypeMentioned      NotificationType = "mentioned"
)

type Notification struct {
	UUID           string
	Type           NotificationType
	ActorUUID      string
	PostUUID       string
	CommentUUID    *string
	ActorName      string
	PostTitle      string
	CommentPreview string
	ReadAt         *time.Time
	CreatedAt      time.Time
}

type NotificationList struct {
	Notifications []Notification
	Limit         int
	Cursor        string
	HasMore       bool
	NextCursor    *string
}
