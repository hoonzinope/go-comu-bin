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

func TestReactionService_AddReaction_InvalidTargetType(t *testing.T) {
	repository := newTestRepository()
	userID := seedUser(repository, "user", "pw", "user")
	svc := NewReactionService(repository)

	err := svc.AddReaction(userID, 1, "invalid", "like")
	if !errors.Is(err, customError.ErrInternalServerError) {
		t.Fatalf("expected ErrInternalServerError, got: %v", err)
	}
}

func TestReactionService_GetReactionsByTarget_AndOwnerDelete(t *testing.T) {
	repository := newTestRepository()
	userID := seedUser(repository, "user", "pw", "user")
	boardID := seedBoard(repository, "free", "desc")
	postID := seedPost(repository, userID, boardID, "title", "content")
	commentID := seedComment(repository, userID, postID, "comment")
	svc := NewReactionService(repository)

	if err := svc.AddReaction(userID, commentID, "comment", "like"); err != nil {
		t.Fatalf("AddReaction returned error: %v", err)
	}
	reactions, err := svc.GetReactionsByTarget(commentID, "comment")
	if err != nil {
		t.Fatalf("GetReactionsByTarget returned error: %v", err)
	}
	if len(reactions) != 1 {
		t.Fatalf("expected 1 reaction, got %d", len(reactions))
	}

	if err := svc.RemoveReaction(userID, reactions[0].ID); err != nil {
		t.Fatalf("RemoveReaction returned error: %v", err)
	}
}
