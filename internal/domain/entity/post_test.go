package entity

import (
	"testing"
	"time"
)

func TestPost_NewPostAndUpdatePost(t *testing.T) {
	p := &Post{}
	p.NewPost("title", "content", 10, 20)

	if p.Title != "title" || p.Content != "content" || p.AuthorID != 10 || p.BoardID != 20 {
		t.Fatalf("unexpected post fields: %+v", p)
	}
	if p.CreatedAt.IsZero() || p.UpdatedAt.IsZero() {
		t.Fatal("expected non-zero CreatedAt/UpdatedAt")
	}

	before := p.UpdatedAt
	time.Sleep(time.Millisecond)
	p.UpdatePost("new-title", "new-content")

	if p.Title != "new-title" || p.Content != "new-content" {
		t.Fatalf("unexpected updated post fields: %+v", p)
	}
	if !p.UpdatedAt.After(before) {
		t.Fatalf("expected UpdatedAt to increase: before=%v after=%v", before, p.UpdatedAt)
	}
}
