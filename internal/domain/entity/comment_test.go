package entity

import "testing"

func TestComment_NewCommentAndUpdateComment(t *testing.T) {
	parentID := int64(9)
	c := &Comment{}
	c.NewComment("hello", 1, 2, &parentID)

	if c.Content != "hello" || c.AuthorID != 1 || c.PostID != 2 {
		t.Fatalf("unexpected comment fields: %+v", c)
	}
	if c.ParentID == nil || *c.ParentID != parentID {
		t.Fatalf("unexpected parent id: %+v", c.ParentID)
	}
	if c.CreatedAt.IsZero() {
		t.Fatal("expected non-zero CreatedAt")
	}

	c.UpdateComment("updated")
	if c.Content != "updated" {
		t.Fatalf("unexpected updated content: %s", c.Content)
	}
}
