package service

import (
	"errors"
	"testing"

	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
)

func TestBoardService_CreateBoard_ForbiddenForNonAdmin(t *testing.T) {
	repository := newTestRepository()
	userID := seedUser(repository, "user", "pw", "user")
	svc := NewBoardService(repository)

	_, err := svc.CreateBoard(userID, "free", "desc")
	if !errors.Is(err, customError.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got: %v", err)
	}
}

func TestBoardService_CreateBoard_SuccessForAdmin(t *testing.T) {
	repository := newTestRepository()
	adminID := seedUser(repository, "admin", "pw", "admin")
	svc := NewBoardService(repository)

	boardID, err := svc.CreateBoard(adminID, "free", "desc")
	if err != nil {
		t.Fatalf("CreateBoard returned error: %v", err)
	}
	if boardID == 0 {
		t.Fatal("expected non-zero boardID")
	}
}
