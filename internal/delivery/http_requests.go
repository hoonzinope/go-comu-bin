package delivery

import (
	"errors"

	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type userCredentialRequest struct {
	Username string `json:"username" example:"alice"`
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
	Title   string `json:"title" example:"hello"`
	Content string `json:"content" example:"first post"`
}

type commentRequest struct {
	Content string `json:"content" example:"nice post"`
}

type reactionRequest struct {
	ReactionType string `json:"reaction_type" example:"like"`
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
	return nil
}

func (r reactionRequest) parseType() (entity.ReactionType, error) {
	if r.ReactionType == "" {
		return "", errors.New("reaction_type is required")
	}
	reactionType, ok := entity.ParseReactionType(r.ReactionType)
	if !ok {
		return "", errors.New("invalid reaction_type")
	}
	return reactionType, nil
}

func (r userSuspensionRequest) parse() (string, entity.SuspensionDuration, error) {
	if r.Reason == "" {
		return "", "", errors.New("reason is required")
	}
	if r.Duration == "" {
		return "", "", errors.New("duration is required")
	}
	duration, ok := entity.ParseSuspensionDuration(r.Duration)
	if !ok {
		return "", "", errors.New("invalid duration")
	}
	return r.Reason, duration, nil
}
