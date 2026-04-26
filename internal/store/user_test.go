package store

import (
	"testing"
)

func TestCreateUser_And_GetByUsername(t *testing.T) {
	st := openTestDB(t)

	id, err := st.CreateUser("alice", "$2a$10$fakehash", "reader")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if id == 0 {
		t.Error("want non-zero ID")
	}

	u, err := st.GetUserByUsername("alice")
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}
	if u.Username != "alice" {
		t.Errorf("Username: want alice, got %q", u.Username)
	}
	if u.Role != "reader" {
		t.Errorf("Role: want reader, got %q", u.Role)
	}
	if u.PasswordHash != "$2a$10$fakehash" {
		t.Errorf("PasswordHash mismatch")
	}
	if !u.IsActive {
		t.Error("want IsActive=true by default")
	}
}

func TestToggleUser(t *testing.T) {
	st := openTestDB(t)
	id, _ := st.CreateUser("bob", "hash", "reader")

	u, _ := st.GetUser(id)
	if !u.IsActive {
		t.Fatal("want IsActive=true initially")
	}

	if err := st.ToggleUser(id); err != nil {
		t.Fatalf("first ToggleUser: %v", err)
	}
	u, _ = st.GetUser(id)
	if u.IsActive {
		t.Error("want IsActive=false after first toggle")
	}

	if err := st.ToggleUser(id); err != nil {
		t.Fatalf("second ToggleUser: %v", err)
	}
	u, _ = st.GetUser(id)
	if !u.IsActive {
		t.Error("want IsActive=true after second toggle")
	}
}

func TestUpdateUserPassword(t *testing.T) {
	st := openTestDB(t)
	id, _ := st.CreateUser("carol", "old-hash", "reader")

	if err := st.UpdateUserPassword(id, "new-hash"); err != nil {
		t.Fatalf("UpdateUserPassword: %v", err)
	}
	u, _ := st.GetUser(id)
	if u.PasswordHash != "new-hash" {
		t.Errorf("PasswordHash: want new-hash, got %q", u.PasswordHash)
	}
}

func TestUpdateUserRole(t *testing.T) {
	st := openTestDB(t)
	id, _ := st.CreateUser("dave", "hash", "reader")

	if err := st.UpdateUserRole(id, "admin"); err != nil {
		t.Fatalf("UpdateUserRole: %v", err)
	}
	u, _ := st.GetUser(id)
	if u.Role != "admin" {
		t.Errorf("Role: want admin, got %q", u.Role)
	}
}
