package inmemory

import "testing"

func TestReactionRepository_AddGetRemove(t *testing.T) {
	repo := NewReactionRepository()
	reaction := testReaction("post", 10, "like", 1)

	if err := repo.Add(reaction); err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	if reaction.ID == 0 {
		t.Fatal("expected non-zero reaction id")
	}

	byID, err := repo.GetByID(reaction.ID)
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if byID == nil || byID.Type != "like" {
		t.Fatalf("unexpected reaction by id: %+v", byID)
	}

	list, err := repo.GetByTarget(10, "post")
	if err != nil {
		t.Fatalf("GetByTarget returned error: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 reaction, got %d", len(list))
	}

	if err := repo.Remove(reaction); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}
	deleted, _ := repo.GetByID(reaction.ID)
	if deleted != nil {
		t.Fatalf("expected nil after remove, got %+v", deleted)
	}
}

func TestReactionRepository_GetMissing_ReturnsNil(t *testing.T) {
	repo := NewReactionRepository()

	r, err := repo.GetByID(999)
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if r != nil {
		t.Fatalf("expected nil for missing reaction, got %+v", r)
	}
}
