package event

import (
	"time"

	"github.com/google/uuid"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

const (
	EventNameBoardChanged                     = "board.changed"
	EventNamePostChanged                      = "post.changed"
	EventNameCommentChanged                   = "comment.changed"
	EventNameReactionChanged                  = "reaction.changed"
	EventNameAttachmentChanged                = "attachment.changed"
	EventNameReportChanged                    = "report.changed"
	EventNameNotificationTriggered            = "notification.triggered"
	EventNameSignupEmailVerificationRequested = "email.verification.signup.requested"
	EventNameEmailVerificationResendRequested = "email.verification.resend.requested"
	EventNamePasswordResetRequested           = "password.reset.requested"
)

type BoardChanged struct {
	Operation string    `json:"operation"`
	BoardID   int64     `json:"board_id"`
	At        time.Time `json:"occurred_at"`
}

func NewBoardChanged(operation string, boardID int64) BoardChanged {
	return BoardChanged{Operation: operation, BoardID: boardID, At: time.Now()}
}

func (e BoardChanged) EventName() string {
	return EventNameBoardChanged
}

func (e BoardChanged) OccurredAt() time.Time {
	return e.At
}

type PostChanged struct {
	Operation         string     `json:"operation"`
	PostID            int64      `json:"post_id"`
	BoardID           int64      `json:"board_id"`
	PublishedAt       *time.Time `json:"published_at,omitempty"`
	TagNames          []string   `json:"tag_names,omitempty"`
	DeletedCommentIDs []int64    `json:"deleted_comment_ids,omitempty"`
	At                time.Time  `json:"occurred_at"`
}

func NewPostChanged(operation string, postID, boardID int64, publishedAt *time.Time, tagNames []string, deletedCommentIDs []int64) PostChanged {
	return PostChanged{Operation: operation, PostID: postID, BoardID: boardID, PublishedAt: publishedAt, TagNames: tagNames, DeletedCommentIDs: deletedCommentIDs, At: time.Now()}
}

func (e PostChanged) EventName() string {
	return EventNamePostChanged
}

func (e PostChanged) OccurredAt() time.Time {
	return e.At
}

type CommentChanged struct {
	Operation string    `json:"operation"`
	CommentID int64     `json:"comment_id"`
	PostID    int64     `json:"post_id"`
	At        time.Time `json:"occurred_at"`
}

func NewCommentChanged(operation string, commentID, postID int64) CommentChanged {
	return CommentChanged{Operation: operation, CommentID: commentID, PostID: postID, At: time.Now()}
}

func (e CommentChanged) EventName() string {
	return EventNameCommentChanged
}

func (e CommentChanged) OccurredAt() time.Time {
	return e.At
}

type ReactionChanged struct {
	Operation    string                    `json:"operation"`
	TargetType   entity.ReactionTargetType `json:"target_type"`
	TargetID     int64                     `json:"target_id"`
	PostID       int64                     `json:"post_id"`
	UserID       int64                     `json:"user_id"`
	ReactionType entity.ReactionType       `json:"reaction_type"`
	At           time.Time                 `json:"occurred_at"`
}

func NewReactionChanged(operation string, targetType entity.ReactionTargetType, targetID, postID, userID int64, reactionType entity.ReactionType) ReactionChanged {
	return ReactionChanged{Operation: operation, TargetType: targetType, TargetID: targetID, PostID: postID, UserID: userID, ReactionType: reactionType, At: time.Now()}
}

func (e ReactionChanged) EventName() string {
	return EventNameReactionChanged
}

func (e ReactionChanged) OccurredAt() time.Time {
	return e.At
}

type AttachmentChanged struct {
	Operation    string    `json:"operation"`
	AttachmentID int64     `json:"attachment_id"`
	PostID       int64     `json:"post_id"`
	At           time.Time `json:"occurred_at"`
}

func NewAttachmentChanged(operation string, attachmentID, postID int64) AttachmentChanged {
	return AttachmentChanged{Operation: operation, AttachmentID: attachmentID, PostID: postID, At: time.Now()}
}

func (e AttachmentChanged) EventName() string {
	return EventNameAttachmentChanged
}

func (e AttachmentChanged) OccurredAt() time.Time {
	return e.At
}

type ReportChanged struct {
	Operation string    `json:"operation"`
	ReportID  int64     `json:"report_id"`
	Status    string    `json:"status"`
	At        time.Time `json:"occurred_at"`
}

func NewReportChanged(operation string, reportID int64, status string) ReportChanged {
	return ReportChanged{Operation: operation, ReportID: reportID, Status: status, At: time.Now()}
}

func (e ReportChanged) EventName() string {
	return EventNameReportChanged
}

func (e ReportChanged) OccurredAt() time.Time {
	return e.At
}

type NotificationTriggered struct {
	EventID                string                  `json:"event_id"`
	RecipientUserID        int64                   `json:"recipient_user_id"`
	ActorUserID            int64                   `json:"actor_user_id"`
	Type                   entity.NotificationType `json:"type"`
	PostID                 int64                   `json:"post_id"`
	CommentID              int64                   `json:"comment_id"`
	ActorNameSnapshot      string                  `json:"actor_name_snapshot"`
	PostTitleSnapshot      string                  `json:"post_title_snapshot"`
	CommentPreviewSnapshot string                  `json:"comment_preview_snapshot"`
	At                     time.Time               `json:"occurred_at"`
}

func NewNotificationTriggered(recipientUserID, actorUserID int64, notificationType entity.NotificationType, postID, commentID int64, actorNameSnapshot, postTitleSnapshot, commentPreviewSnapshot string) NotificationTriggered {
	return NotificationTriggered{
		EventID:                uuid.NewString(),
		RecipientUserID:        recipientUserID,
		ActorUserID:            actorUserID,
		Type:                   notificationType,
		PostID:                 postID,
		CommentID:              commentID,
		ActorNameSnapshot:      actorNameSnapshot,
		PostTitleSnapshot:      postTitleSnapshot,
		CommentPreviewSnapshot: commentPreviewSnapshot,
		At:                     time.Now(),
	}
}

func (e NotificationTriggered) EventName() string {
	return EventNameNotificationTriggered
}

func (e NotificationTriggered) OccurredAt() time.Time {
	return e.At
}

type MailDeliveryRequested struct {
	UserID    int64     `json:"user_id"`
	Email     string    `json:"email"`
	RawToken  string    `json:"raw_token"`
	TokenHash string    `json:"token_hash"`
	ExpiresAt time.Time `json:"expires_at"`
	At        time.Time `json:"occurred_at"`
}

type SignupEmailVerificationRequested struct {
	MailDeliveryRequested
}

func NewSignupEmailVerificationRequested(userID int64, email, rawToken, tokenHash string, expiresAt time.Time) SignupEmailVerificationRequested {
	return SignupEmailVerificationRequested{
		MailDeliveryRequested: MailDeliveryRequested{
			UserID:    userID,
			Email:     email,
			RawToken:  rawToken,
			TokenHash: tokenHash,
			ExpiresAt: expiresAt,
			At:        time.Now(),
		},
	}
}

func (e SignupEmailVerificationRequested) EventName() string {
	return EventNameSignupEmailVerificationRequested
}

func (e SignupEmailVerificationRequested) OccurredAt() time.Time {
	return e.At
}

type EmailVerificationResendRequested struct {
	MailDeliveryRequested
}

func NewEmailVerificationResendRequested(userID int64, email, rawToken, tokenHash string, expiresAt time.Time) EmailVerificationResendRequested {
	return EmailVerificationResendRequested{
		MailDeliveryRequested: MailDeliveryRequested{
			UserID:    userID,
			Email:     email,
			RawToken:  rawToken,
			TokenHash: tokenHash,
			ExpiresAt: expiresAt,
			At:        time.Now(),
		},
	}
}

func (e EmailVerificationResendRequested) EventName() string {
	return EventNameEmailVerificationResendRequested
}

func (e EmailVerificationResendRequested) OccurredAt() time.Time {
	return e.At
}

type PasswordResetRequested struct {
	MailDeliveryRequested
}

func NewPasswordResetRequested(userID int64, email, rawToken, tokenHash string, expiresAt time.Time) PasswordResetRequested {
	return PasswordResetRequested{
		MailDeliveryRequested: MailDeliveryRequested{
			UserID:    userID,
			Email:     email,
			RawToken:  rawToken,
			TokenHash: tokenHash,
			ExpiresAt: expiresAt,
			At:        time.Now(),
		},
	}
}

func (e PasswordResetRequested) EventName() string {
	return EventNamePasswordResetRequested
}

func (e PasswordResetRequested) OccurredAt() time.Time {
	return e.At
}
