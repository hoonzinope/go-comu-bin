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

type userSuspensionResponse struct {
	UserID         int64      `json:"user_id" example:"1"`
	Status         string     `json:"status" example:"suspended"`
	Reason         string     `json:"reason,omitempty" example:"spam"`
	SuspendedUntil *time.Time `json:"suspended_until,omitempty" example:"2026-03-15T10:00:00Z"`
}

type attachmentListResponse struct {
	Attachments []response.Attachment `json:"attachments"`
}

type attachmentUploadResponse struct {
	ID            int64  `json:"id" example:"1"`
	EmbedMarkdown string `json:"embed_markdown" example:"![a.png](attachment://1)"`
}
