package inmemory

import "testing"

func TestPostRepository_FilterByBoardAndPagination(t *testing.T) {
	repo := NewPostRepository()
	_, _ = repo.Save(testPost("p1", "c1", 1, 1))
	_, _ = repo.Save(testPost("p2", "c2", 1, 1))
	_, _ = repo.Save(testPost("p3", "c3", 2, 2))

	posts, err := repo.SelectPosts(1, 10, 0)
	if err != nil {
		t.Fatalf("SelectPosts returned error: %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("expected 2 posts for board 1, got %d", len(posts))
	}
}

func TestPostRepository_SaveSelectUpdateDelete(t *testing.T) {
	repo := NewPostRepository()
	post := testPost("title", "content", 1, 1)
	id, err := repo.Save(post)
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	selected, err := repo.SelectPostByID(id)
	if err != nil {
		t.Fatalf("SelectPostByID returned error: %v", err)
	}
	if selected == nil || selected.Title != "title" {
		t.Fatalf("unexpected post: %+v", selected)
	}

	selected.UpdatePost("new", "new-content")
	if err := repo.Update(selected); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	updated, _ := repo.SelectPostByID(id)
	if updated.Title != "new" {
		t.Fatalf("expected updated title, got %s", updated.Title)
	}

	if err := repo.Delete(id); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	deleted, _ := repo.SelectPostByID(id)
	if deleted != nil {
		t.Fatalf("expected nil after delete, got %+v", deleted)
	}
}

func TestPostRepository_PaginationOffsetEqualsLen_ReturnsEmpty(t *testing.T) {
	repo := NewPostRepository()
	_, _ = repo.Save(testPost("p1", "c1", 1, 1))
	_, _ = repo.Save(testPost("p2", "c2", 1, 1))

	posts, err := repo.SelectPosts(1, 10, 2)
	if err != nil {
		t.Fatalf("SelectPosts returned error: %v", err)
	}
	if len(posts) != 0 {
		t.Fatalf("expected empty result, got %d", len(posts))
	}
}

func TestPostRepository_UpdateDelete_NonExistingID_NoError(t *testing.T) {
	repo := NewPostRepository()
	p := testPost("x", "y", 1, 1)
	p.ID = 999

	if err := repo.Update(p); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if err := repo.Delete(999); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
}
