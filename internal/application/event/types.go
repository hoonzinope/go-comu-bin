package event

import (
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

const (
	EventNameBoardChanged      = "board.changed"
	EventNamePostChanged       = "post.changed"
	EventNameCommentChanged    = "comment.changed"
	EventNameReactionChanged   = "reaction.changed"
	EventNameAttachmentChanged = "attachment.changed"
	EventNameReportChanged     = "report.changed"
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
	Operation         string    `json:"operation"`
	PostID            int64     `json:"post_id"`
	BoardID           int64     `json:"board_id"`
	TagNames          []string  `json:"tag_names,omitempty"`
	DeletedCommentIDs []int64   `json:"deleted_comment_ids,omitempty"`
	At                time.Time `json:"occurred_at"`
}

func NewPostChanged(operation string, postID, boardID int64, tagNames []string, deletedCommentIDs []int64) PostChanged {
	return PostChanged{Operation: operation, PostID: postID, BoardID: boardID, TagNames: tagNames, DeletedCommentIDs: deletedCommentIDs, At: time.Now()}
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
	Operation  string                    `json:"operation"`
	TargetType entity.ReactionTargetType `json:"target_type"`
	TargetID   int64                     `json:"target_id"`
	PostID     int64                     `json:"post_id"`
	At         time.Time                 `json:"occurred_at"`
}

func NewReactionChanged(operation string, targetType entity.ReactionTargetType, targetID, postID int64) ReactionChanged {
	return ReactionChanged{Operation: operation, TargetType: targetType, TargetID: targetID, PostID: postID, At: time.Now()}
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
