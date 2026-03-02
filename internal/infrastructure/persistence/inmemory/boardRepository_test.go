package inmemory

import "testing"

func TestBoardRepository_ListPagination(t *testing.T) {
	repo := NewBoardRepository()
	b1 := testBoard("b1", "d1")
	b2 := testBoard("b2", "d2")
	b3 := testBoard("b3", "d3")
	_, _ = repo.Save(b1)
	_, _ = repo.Save(b2)
	_, _ = repo.Save(b3)

	boards, err := repo.SelectBoardList(2, 1)
	if err != nil {
		t.Fatalf("SelectBoardList returned error: %v", err)
	}
	if len(boards) != 2 {
		t.Fatalf("expected 2 boards, got %d", len(boards))
	}
}

func TestBoardRepository_SaveSelectUpdateDelete(t *testing.T) {
	repo := NewBoardRepository()
	board := testBoard("free", "desc")
	id, err := repo.Save(board)
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	selected, err := repo.SelectBoardByID(id)
	if err != nil {
		t.Fatalf("SelectBoardByID returned error: %v", err)
	}
	if selected == nil || selected.Name != "free" {
		t.Fatalf("unexpected board: %+v", selected)
	}

	selected.UpdateBoard("new", "new-desc")
	if err := repo.Update(selected); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	updated, _ := repo.SelectBoardByID(id)
	if updated.Name != "new" {
		t.Fatalf("expected updated name, got %s", updated.Name)
	}

	if err := repo.Delete(id); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	deleted, _ := repo.SelectBoardByID(id)
	if deleted != nil {
		t.Fatalf("expected nil after delete, got %+v", deleted)
	}
}

func TestBoardRepository_PaginationOffsetEqualsLen_ReturnsEmpty(t *testing.T) {
	repo := NewBoardRepository()
	_, _ = repo.Save(testBoard("b1", "d1"))
	_, _ = repo.Save(testBoard("b2", "d2"))

	boards, err := repo.SelectBoardList(10, 2)
	if err != nil {
		t.Fatalf("SelectBoardList returned error: %v", err)
	}
	if len(boards) != 0 {
		t.Fatalf("expected empty result, got %d", len(boards))
	}
}

func TestBoardRepository_UpdateDelete_NonExistingID_NoError(t *testing.T) {
	repo := NewBoardRepository()
	b := testBoard("x", "y")
	b.ID = 999

	if err := repo.Update(b); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if err := repo.Delete(999); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
}
