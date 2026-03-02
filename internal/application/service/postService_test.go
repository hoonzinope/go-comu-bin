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

func TestPostService_CreateGetListDelete_Success(t *testing.T) {
	repository := newTestRepository()
	userID := seedUser(repository, "user", "pw", "user")
	boardID := seedBoard(repository, "free", "desc")
	svc := NewPostService(repository)

	postID, err := svc.CreatePost("title", "content", userID, boardID)
	if err != nil {
		t.Fatalf("CreatePost returned error: %v", err)
	}
	if postID == 0 {
		t.Fatal("expected non-zero postID")
	}

	list, err := svc.GetPostsList(boardID, 10, 0)
	if err != nil {
		t.Fatalf("GetPostsList returned error: %v", err)
	}
	if len(list.Posts) != 1 {
		t.Fatalf("expected 1 post, got %d", len(list.Posts))
	}

	detail, err := svc.GetPostDetail(postID)
	if err != nil {
		t.Fatalf("GetPostDetail returned error: %v", err)
	}
	if detail.Post == nil || detail.Post.ID != postID {
		t.Fatalf("unexpected post detail: %+v", detail.Post)
	}

	if err := svc.DeletePost(postID, userID); err != nil {
		t.Fatalf("DeletePost returned error: %v", err)
	}
}
