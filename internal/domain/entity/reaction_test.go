package entity

import "testing"

func TestReaction_NewReactionAndUpdateReaction(t *testing.T) {
	r := &Reaction{}
	r.NewReaction("post", 3, "like", 7)

	if r.TargetType != "post" || r.TargetID != 3 || r.Type != "like" || r.UserID != 7 {
		t.Fatalf("unexpected reaction fields: %+v", r)
	}
	if r.CreatedAt.IsZero() {
		t.Fatal("expected non-zero CreatedAt")
	}

	r.UpdateReaction("dislike")
	if r.Type != "dislike" {
		t.Fatalf("expected dislike, got %s", r.Type)
	}
}
