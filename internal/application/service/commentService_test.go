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

func TestCommentService_CreateGetDelete_Success(t *testing.T) {
	repository := newTestRepository()
	userID := seedUser(repository, "user", "pw", "user")
	boardID := seedBoard(repository, "free", "desc")
	postID := seedPost(repository, userID, boardID, "title", "content")
	svc := NewCommentService(repository)

	commentID, err := svc.CreateComment("comment", userID, postID)
	if err != nil {
		t.Fatalf("CreateComment returned error: %v", err)
	}
	if commentID == 0 {
		t.Fatal("expected non-zero commentID")
	}

	list, err := svc.GetCommentsByPost(postID, 10, 0)
	if err != nil {
		t.Fatalf("GetCommentsByPost returned error: %v", err)
	}
	if len(list.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(list.Comments))
	}

	if err := svc.DeleteComment(commentID, userID); err != nil {
		t.Fatalf("DeleteComment returned error: %v", err)
	}
}
