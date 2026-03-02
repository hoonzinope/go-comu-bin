package entity

import "testing"

func TestUser_NewUserAndIsAdmin(t *testing.T) {
	u := &User{}
	u.NewUser("alice", "pw")

	if u.Name != "alice" || u.Password != "pw" {
		t.Fatalf("unexpected user fields: %+v", u)
	}
	if u.Role != "user" {
		t.Fatalf("expected role user, got %s", u.Role)
	}
	if u.IsAdmin() {
		t.Fatal("expected non-admin user")
	}
	if u.CreatedAt.IsZero() {
		t.Fatal("expected non-zero CreatedAt")
	}
}

func TestUser_NewAdminAndIsAdmin(t *testing.T) {
	u := &User{}
	u.NewAdmin("admin", "pw")

	if u.Role != "admin" {
		t.Fatalf("expected role admin, got %s", u.Role)
	}
	if !u.IsAdmin() {
		t.Fatal("expected admin user")
	}
	if u.CreatedAt.IsZero() {
		t.Fatal("expected non-zero CreatedAt")
	}
}
