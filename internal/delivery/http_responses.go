package delivery

import (
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/delivery/response"
)

type signUpResponse struct {
	Result string `json:"result" example:"ok"`
}

type loginResponse struct {
	Login string `json:"login" example:"ok"`
}

type logoutResponse struct {
	Logout string `json:"logout" example:"ok"`
}

type errorResponse struct {
	Error string `json:"error" example:"invalid credential"`
}

type idResponse struct {
	ID int64 `json:"id" example:"1"`
}

type uuidResponse struct {
	UUID string `json:"uuid" example:"550e8400-e29b-41d4-a716-446655440000"`
}

type userSuspensionResponse struct {
	UserUUID       string     `json:"user_uuid" example:"550e8400-e29b-41d4-a716-446655440000"`
	Status         string     `json:"status" example:"suspended"`
	Reason         string     `json:"reason,omitempty" example:"spam"`
	SuspendedUntil *time.Time `json:"suspended_until,omitempty" example:"2026-03-15T10:00:00Z"`
}

type attachmentListResponse struct {
	Attachments []response.Attachment `json:"attachments"`
}

type attachmentUploadResponse struct {
	UUID          string `json:"uuid" example:"550e8400-e29b-41d4-a716-446655440000"`
	EmbedMarkdown string `json:"embed_markdown" example:"![a.png](attachment://550e8400-e29b-41d4-a716-446655440000)"`
	PreviewURL    string `json:"preview_url" example:"/api/v1/posts/550e8400-e29b-41d4-a716-446655440000/attachments/550e8400-e29b-41d4-a716-446655440000/preview"`
}

type reportResponse struct {
	ID             int64      `json:"id"`
	TargetType     string     `json:"target_type"`
	TargetID       int64      `json:"target_id"`
	ReporterUserID int64      `json:"reporter_user_id"`
	ReporterUUID   string     `json:"reporter_uuid"`
	ReasonCode     string     `json:"reason_code"`
	ReasonDetail   string     `json:"reason_detail,omitempty"`
	Status         string     `json:"status"`
	ResolutionNote string     `json:"resolution_note,omitempty"`
	ResolvedBy     *int64     `json:"resolved_by,omitempty"`
	ResolvedByUUID *string    `json:"resolved_by_uuid,omitempty"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type reportListResponse struct {
	Reports    []reportResponse `json:"reports"`
	Limit      int              `json:"limit"`
	LastID     int64            `json:"last_id"`
	HasMore    bool             `json:"has_more"`
	NextLastID *int64           `json:"next_last_id,omitempty"`
}

type outboxDeadMessageResponse struct {
	ID            string    `json:"id"`
	EventName     string    `json:"event_name"`
	AttemptCount  int       `json:"attempt_count"`
	LastError     string    `json:"last_error"`
	OccurredAt    time.Time `json:"occurred_at"`
	NextAttemptAt time.Time `json:"next_attempt_at"`
}

type outboxDeadListResponse struct {
	Messages   []outboxDeadMessageResponse `json:"messages"`
	Limit      int                         `json:"limit"`
	LastID     string                      `json:"last_id"`
	HasMore    bool                        `json:"has_more"`
	NextLastID *string                     `json:"next_last_id,omitempty"`
}
