package delivery

import (
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/hoonzinope/go-comu-bin/internal/application/model"
)

type userCredentialRequest struct {
	Username string `json:"username" example:"alice"`
	Password string `json:"password" example:"pw"`
}

type guestUpgradeRequest struct {
	Username string `json:"username" example:"alice"`
	Email    string `json:"email" example:"alice@example.com"`
	Password string `json:"password" example:"pw"`
}

type passwordOnlyRequest struct {
	Password string `json:"password" example:"pw"`
}

type userSuspensionRequest struct {
	Reason   string `json:"reason" example:"spam"`
	Duration string `json:"duration" example:"7d"`
}

type boardRequest struct {
	Name        string `json:"name" example:"free"`
	Description string `json:"description" example:"free board"`
}

type postRequest struct {
	Title              string   `json:"title" example:"hello"`
	Content            string   `json:"content" example:"first post"`
	Tags               []string `json:"tags,omitempty" example:"go,backend"`
	MentionedUsernames []string `json:"mentioned_usernames,omitempty" example:"alice,bob"`
}

type commentRequest struct {
	Content            string   `json:"content" example:"nice post"`
	ParentUUID         *string  `json:"parent_uuid,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	MentionedUsernames []string `json:"mentioned_usernames,omitempty" example:"alice,bob"`
}

type reactionRequest struct {
	ReactionType string `json:"reaction_type" example:"like"`
}

type reportCreateRequest struct {
	TargetType   string `json:"target_type" example:"post"`
	TargetUUID   string `json:"target_uuid" example:"550e8400-e29b-41d4-a716-446655440000"`
	ReasonCode   string `json:"reason_code" example:"spam"`
	ReasonDetail string `json:"reason_detail,omitempty" example:"repeated spam"`
}

type reportResolveRequest struct {
	Status         string `json:"status" example:"accepted"`
	ResolutionNote string `json:"resolution_note,omitempty" example:"confirmed"`
}

type boardVisibilityRequest struct {
	Hidden bool `json:"hidden"`
}

func (r userCredentialRequest) validate() error {
	if r.Username == "" || r.Password == "" {
		return errors.New("username and password are required")
	}
	return nil
}

func (r passwordOnlyRequest) validate() error {
	if r.Password == "" {
		return errors.New("password is required")
	}
	return nil
}

func (r guestUpgradeRequest) validate() error {
	if strings.TrimSpace(r.Username) == "" || strings.TrimSpace(r.Email) == "" || strings.TrimSpace(r.Password) == "" {
		return errors.New("username, email and password are required")
	}
	return nil
}

func (r boardRequest) validate() error {
	if r.Name == "" {
		return errors.New("name is required")
	}
	return nil
}

func (r postRequest) validate() error {
	if r.Title == "" || r.Content == "" {
		return errors.New("title and content are required")
	}
	return nil
}

func (r commentRequest) validate() error {
	if r.Content == "" {
		return errors.New("content is required")
	}
	if r.ParentUUID != nil && strings.TrimSpace(*r.ParentUUID) != "" {
		if _, err := uuid.Parse(strings.TrimSpace(*r.ParentUUID)); err != nil {
			return errors.New("invalid parent_uuid")
		}
	}
	return nil
}

func (r reactionRequest) parseType() (model.ReactionType, error) {
	if r.ReactionType == "" {
		return "", errors.New("reaction_type is required")
	}
	reactionType, ok := model.ParseReactionType(r.ReactionType)
	if !ok {
		return "", errors.New("invalid reaction_type")
	}
	return reactionType, nil
}

func (r userSuspensionRequest) parse() (string, model.SuspensionDuration, error) {
	if r.Reason == "" {
		return "", "", errors.New("reason is required")
	}
	if r.Duration == "" {
		return "", "", errors.New("duration is required")
	}
	duration, ok := model.ParseSuspensionDuration(r.Duration)
	if !ok {
		return "", "", errors.New("invalid duration")
	}
	return r.Reason, duration, nil
}

func (r reportCreateRequest) parse() (model.ReportTargetType, string, model.ReportReasonCode, string, error) {
	targetType, ok := model.ParseReportTargetType(r.TargetType)
	if !ok {
		return "", "", "", "", errors.New("invalid target_type")
	}
	if strings.TrimSpace(r.TargetUUID) == "" {
		return "", "", "", "", errors.New("invalid target_uuid")
	}
	if _, err := uuid.Parse(strings.TrimSpace(r.TargetUUID)); err != nil {
		return "", "", "", "", errors.New("invalid target_uuid")
	}
	reasonCode, ok := model.ParseReportReasonCode(r.ReasonCode)
	if !ok {
		return "", "", "", "", errors.New("invalid reason_code")
	}
	return targetType, strings.TrimSpace(r.TargetUUID), reasonCode, r.ReasonDetail, nil
}

func (r reportResolveRequest) parseStatus() (model.ReportStatus, string, error) {
	status, ok := model.ParseReportStatus(r.Status)
	if !ok {
		return "", "", errors.New("invalid status")
	}
	if status != model.ReportStatusAccepted && status != model.ReportStatusRejected {
		return "", "", errors.New("status must be accepted or rejected")
	}
	return status, r.ResolutionNote, nil
}
