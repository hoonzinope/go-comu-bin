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

func TestBoardService_GetBoards_Success(t *testing.T) {
	repository := newTestRepository()
	seedBoard(repository, "b1", "d1")
	seedBoard(repository, "b2", "d2")
	svc := NewBoardService(repository)

	list, err := svc.GetBoards(10, 0)
	if err != nil {
		t.Fatalf("GetBoards returned error: %v", err)
	}
	if len(list.Boards) != 2 {
		t.Fatalf("expected 2 boards, got %d", len(list.Boards))
	}
}

func TestBoardService_UpdateDelete_SuccessForAdmin(t *testing.T) {
	repository := newTestRepository()
	adminID := seedUser(repository, "admin", "pw", "admin")
	boardID := seedBoard(repository, "free", "desc")
	svc := NewBoardService(repository)

	if err := svc.UpdateBoard(boardID, adminID, "new", "new-desc"); err != nil {
		t.Fatalf("UpdateBoard returned error: %v", err)
	}
	if err := svc.DeleteBoard(boardID, adminID); err != nil {
		t.Fatalf("DeleteBoard returned error: %v", err)
	}
}
