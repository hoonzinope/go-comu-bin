package service

import (
	"errors"
	"testing"

	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
)

func TestPostService_UpdatePost_ForbiddenForNonOwnerNonAdmin(t *testing.T) {
	repository := newTestRepository()
	ownerID := seedUser(repository, "owner", "pw", "user")
	otherID := seedUser(repository, "other", "pw", "user")
	boardID := seedBoard(repository, "free", "desc")
	postID := seedPost(repository, ownerID, boardID, "title", "content")
	svc := NewPostService(repository)

	err := svc.UpdatePost(postID, otherID, "new-title", "new-content")
	if !errors.Is(err, customError.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got: %v", err)
	}
}

func TestPostService_UpdatePost_AllowedForAdmin(t *testing.T) {
	repository := newTestRepository()
	ownerID := seedUser(repository, "owner", "pw", "user")
	adminID := seedUser(repository, "admin", "pw", "admin")
	boardID := seedBoard(repository, "free", "desc")
	postID := seedPost(repository, ownerID, boardID, "title", "content")
	svc := NewPostService(repository)

	err := svc.UpdatePost(postID, adminID, "new-title", "new-content")
	if err != nil {
		t.Fatalf("UpdatePost returned error: %v", err)
	}
}
