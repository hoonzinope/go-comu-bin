package inmemory

import "testing"

func TestUserRepository_SaveSelectDelete(t *testing.T) {
	repo := NewUserRepository()

	user := testUser("alice", "pw", false)
	id, err := repo.Save(user)
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero id")
	}

	byName, err := repo.SelectUserByUsername("alice")
	if err != nil {
		t.Fatalf("SelectUserByUsername returned error: %v", err)
	}
	if byName == nil || byName.ID != id {
		t.Fatalf("unexpected user by username: %+v", byName)
	}

	byID, err := repo.SelectUserByID(id)
	if err != nil {
		t.Fatalf("SelectUserByID returned error: %v", err)
	}
	if byID == nil || byID.Name != "alice" {
		t.Fatalf("unexpected user by id: %+v", byID)
	}

	if err := repo.Delete(id); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	deleted, err := repo.SelectUserByID(id)
	if err != nil {
		t.Fatalf("SelectUserByID after delete returned error: %v", err)
	}
	if deleted != nil {
		t.Fatalf("expected nil after delete, got: %+v", deleted)
	}
}
