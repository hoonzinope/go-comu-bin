package service

import (
	"errors"
	"testing"

	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
)

func TestCommentService_UpdateComment_ForbiddenForNonOwnerNonAdmin(t *testing.T) {
	repository := newTestRepository()
	ownerID := seedUser(repository, "owner", "pw", "user")
	otherID := seedUser(repository, "other", "pw", "user")
	boardID := seedBoard(repository, "free", "desc")
	postID := seedPost(repository, ownerID, boardID, "title", "content")
	svc := NewCommentService(repository)
	commentID, _ := svc.CreateComment("comment", ownerID, postID)

	err := svc.UpdateComment(commentID, otherID, "new-comment")
	if !errors.Is(err, customError.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got: %v", err)
	}
}

func TestCommentService_UpdateComment_AllowedForAdmin(t *testing.T) {
	repository := newTestRepository()
	ownerID := seedUser(repository, "owner", "pw", "user")
	adminID := seedUser(repository, "admin", "pw", "admin")
	boardID := seedBoard(repository, "free", "desc")
	postID := seedPost(repository, ownerID, boardID, "title", "content")
	svc := NewCommentService(repository)
	commentID, _ := svc.CreateComment("comment", ownerID, postID)

	err := svc.UpdateComment(commentID, adminID, "new-comment")
	if err != nil {
		t.Fatalf("UpdateComment returned error: %v", err)
	}
}
