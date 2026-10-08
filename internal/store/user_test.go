package store

import (
	"context"
	"testing"
)

func TestCreateUser_And_GetByUsername(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)

	id, err := st.CreateUser(ctx, "alice", "$2a$10$fakehash", "reader")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if id == 0 {
		t.Error("want non-zero ID")
	}

	u, err := st.GetUserByUsername(ctx, "alice")
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
	if u.SessionVersion != 1 {
		t.Errorf("SessionVersion: want 1, got %d", u.SessionVersion)
	}
}

func TestToggleUser(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	id, _ := st.CreateUser(ctx, "bob", "hash", "reader")

	u, _ := st.GetUser(ctx, id)
	if !u.IsActive {
		t.Fatal("want IsActive=true initially")
	}

	if err := st.ToggleUser(ctx, id); err != nil {
		t.Fatalf("first ToggleUser: %v", err)
	}
	u, _ = st.GetUser(ctx, id)
	if u.IsActive {
		t.Error("want IsActive=false after first toggle")
	}

	if err := st.ToggleUser(ctx, id); err != nil {
		t.Fatalf("second ToggleUser: %v", err)
	}
	u, _ = st.GetUser(ctx, id)
	if !u.IsActive {
		t.Error("want IsActive=true after second toggle")
	}
}

func TestUpdateUserPassword(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	id, _ := st.CreateUser(ctx, "carol", "old-hash", "reader")

	if err := st.UpdateUserPassword(ctx, id, "new-hash"); err != nil {
		t.Fatalf("UpdateUserPassword: %v", err)
	}
	u, _ := st.GetUser(ctx, id)
	if u.PasswordHash != "new-hash" {
		t.Errorf("PasswordHash: want new-hash, got %q", u.PasswordHash)
	}
	if u.SessionVersion != 2 {
		t.Errorf("SessionVersion: want 2 after password update, got %d", u.SessionVersion)
	}
}

func TestUpdateUserRole(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	id, _ := st.CreateUser(ctx, "dave", "hash", "reader")

	if err := st.UpdateUserRole(ctx, id, "admin"); err != nil {
		t.Fatalf("UpdateUserRole: %v", err)
	}
	u, _ := st.GetUser(ctx, id)
	if u.Role != "admin" {
		t.Errorf("Role: want admin, got %q", u.Role)
	}
	if u.SessionVersion != 2 {
		t.Errorf("SessionVersion: want 2 after role update, got %d", u.SessionVersion)
	}
}

func TestIsLastActiveAdminAndReactivateUser(t *testing.T) {
	st := openTestDB(t)
	ctx := context.Background()

	adminID, err := st.CreateUser(ctx, "admin", "hash", "admin")
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	editorID, err := st.CreateUser(ctx, "editor", "hash", "editor")
	if err != nil {
		t.Fatalf("create editor: %v", err)
	}
	if last, err := st.IsLastActiveAdmin(ctx, adminID); err != nil || !last {
		t.Fatalf("single admin: last=%v err=%v", last, err)
	}
	if last, err := st.IsLastActiveAdmin(ctx, editorID); err != nil || last {
		t.Fatalf("editor reported as last admin: last=%v err=%v", last, err)
	}
	secondID, err := st.CreateUser(ctx, "admin2", "hash", "admin")
	if err != nil {
		t.Fatalf("create second admin: %v", err)
	}
	if last, err := st.IsLastActiveAdmin(ctx, adminID); err != nil || last {
		t.Fatalf("two active admins: last=%v err=%v", last, err)
	}
	if err := st.SetUserActive(ctx, secondID, false); err != nil {
		t.Fatalf("disable second admin: %v", err)
	}
	if last, err := st.IsLastActiveAdmin(ctx, adminID); err != nil || !last {
		t.Fatalf("inactive admins must not count: last=%v err=%v", last, err)
	}

	if err := st.StartUserTOTPGrace(ctx, secondID); err != nil {
		t.Fatalf("start grace: %v", err)
	}
	before, _ := st.GetUser(ctx, secondID)
	if ok, err := st.ReactivateUserByUsername(ctx, "admin2"); err != nil || !ok {
		t.Fatalf("reactivate: ok=%v err=%v", ok, err)
	}
	after, err := st.GetUser(ctx, secondID)
	if err != nil || !after.IsActive || after.TOTPGraceStartedAt != nil || after.SessionVersion <= before.SessionVersion {
		t.Fatalf("reactivated user: %+v %v", after, err)
	}
	if ok, err := st.ReactivateUserByUsername(ctx, "missing"); err != nil || ok {
		t.Fatalf("missing user: ok=%v err=%v", ok, err)
	}
}
