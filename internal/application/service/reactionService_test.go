package service

import (
	"errors"
	"testing"

	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
)

func TestReactionService_RemoveReaction_ForbiddenForNonOwnerNonAdmin(t *testing.T) {
	repository := newTestRepository()
	ownerID := seedUser(repository, "owner", "pw", "user")
	otherID := seedUser(repository, "other", "pw", "user")
	boardID := seedBoard(repository, "free", "desc")
	postID := seedPost(repository, ownerID, boardID, "title", "content")
	svc := NewReactionService(repository)

	if err := svc.AddReaction(ownerID, postID, "post", "like"); err != nil {
		t.Fatalf("AddReaction returned error: %v", err)
	}
	reactions, err := repository.ReactionRepository.GetByTarget(postID, "post")
	if err != nil || len(reactions) != 1 {
		t.Fatalf("failed to prepare reaction: err=%v len=%d", err, len(reactions))
	}

	err = svc.RemoveReaction(otherID, reactions[0].ID)
	if !errors.Is(err, customError.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got: %v", err)
	}
}

func TestReactionService_RemoveReaction_AllowedForAdmin(t *testing.T) {
	repository := newTestRepository()
	ownerID := seedUser(repository, "owner", "pw", "user")
	adminID := seedUser(repository, "admin", "pw", "admin")
	boardID := seedBoard(repository, "free", "desc")
	postID := seedPost(repository, ownerID, boardID, "title", "content")
	svc := NewReactionService(repository)

	if err := svc.AddReaction(ownerID, postID, "post", "like"); err != nil {
		t.Fatalf("AddReaction returned error: %v", err)
	}
	reactions, err := repository.ReactionRepository.GetByTarget(postID, "post")
	if err != nil || len(reactions) != 1 {
		t.Fatalf("failed to prepare reaction: err=%v len=%d", err, len(reactions))
	}

	err = svc.RemoveReaction(adminID, reactions[0].ID)
	if err != nil {
		t.Fatalf("RemoveReaction returned error: %v", err)
	}
}
