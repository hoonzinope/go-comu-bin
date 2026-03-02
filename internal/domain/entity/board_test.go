package entity

import "testing"

func TestBoard_NewBoardAndUpdateBoard(t *testing.T) {
	b := &Board{}
	b.NewBoard("free", "desc")

	if b.Name != "free" || b.Description != "desc" {
		t.Fatalf("unexpected board fields: %+v", b)
	}
	if b.CreatedAt.IsZero() {
		t.Fatal("expected non-zero CreatedAt")
	}

	b.UpdateBoard("notice", "updated")
	if b.Name != "notice" || b.Description != "updated" {
		t.Fatalf("unexpected updated board fields: %+v", b)
	}
}
