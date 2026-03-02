package inmemory

import "testing"

func TestCommentRepository_FilterByPostAndPagination(t *testing.T) {
	repo := NewCommentRepository()
	_, _ = repo.Save(testComment("c1", 1, 1))
	_, _ = repo.Save(testComment("c2", 2, 1))
	_, _ = repo.Save(testComment("c3", 3, 2))

	comments, err := repo.SelectComments(1, 10, 0)
	if err != nil {
		t.Fatalf("SelectComments returned error: %v", err)
	}
	if len(comments) != 2 {
		t.Fatalf("expected 2 comments for post 1, got %d", len(comments))
	}
}

func TestCommentRepository_SaveSelectUpdateDelete(t *testing.T) {
	repo := NewCommentRepository()
	comment := testComment("hello", 1, 1)
	id, err := repo.Save(comment)
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	selected, err := repo.SelectCommentByID(id)
	if err != nil {
		t.Fatalf("SelectCommentByID returned error: %v", err)
	}
	if selected == nil || selected.Content != "hello" {
		t.Fatalf("unexpected comment: %+v", selected)
	}

	selected.UpdateComment("updated")
	if err := repo.Update(selected); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	updated, _ := repo.SelectCommentByID(id)
	if updated.Content != "updated" {
		t.Fatalf("expected updated content, got %s", updated.Content)
	}

	if err := repo.Delete(id); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	deleted, _ := repo.SelectCommentByID(id)
	if deleted != nil {
		t.Fatalf("expected nil after delete, got %+v", deleted)
	}
}
