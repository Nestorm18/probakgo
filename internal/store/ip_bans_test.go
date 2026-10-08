package store

import (
	"context"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestListLoginAttemptsPage(t *testing.T) {
	st := openTestDB(t)
	ctx := context.Background()

	for _, username := range []string{"first", "second", "third"} {
		if err := st.InsertLoginAttempt(ctx, username, "10.0.0.1", "agent", `{"timezone":"Europe/Madrid"}`, "failed", "bad password"); err != nil {
			t.Fatalf("insert login attempt %q: %v", username, err)
		}
	}

	page1, err := st.ListLoginAttemptsPage(ctx, 2, 0)
	if err != nil {
		t.Fatalf("list first page: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("first page rows: got %d, want 2", len(page1))
	}
	if page1[0].Username != "third" || page1[1].Username != "second" {
		t.Fatalf("first page usernames: got %q, %q", page1[0].Username, page1[1].Username)
	}

	page2, err := st.ListLoginAttemptsPage(ctx, 2, 2)
	if err != nil {
		t.Fatalf("list second page: %v", err)
	}
	if len(page2) != 1 {
		t.Fatalf("second page rows: got %d, want 1", len(page2))
	}
	if page2[0].Username != "first" {
		t.Fatalf("second page username: got %q, want first", page2[0].Username)
	}
	if page2[0].ClientDetails != `{"timezone":"Europe/Madrid"}` {
		t.Fatalf("client details: got %q", page2[0].ClientDetails)
	}
}

func TestInsertLoginAttemptBoundsClientValues(t *testing.T) {
	st := openTestDB(t)
	ctx := context.Background()

	username := strings.Repeat("ñ", maxLoginAttemptUsernameLength+50)
	userAgent := strings.Repeat("a", 1<<20)
	if err := st.InsertLoginAttempt(ctx, username, "10.0.0.1", userAgent, "", "failed", "invalid_credentials"); err != nil {
		t.Fatalf("insert login attempt: %v", err)
	}
	attempts, err := st.ListLoginAttemptsPage(ctx, 1, 0)
	if err != nil || len(attempts) != 1 {
		t.Fatalf("list attempts: %v %+v", err, attempts)
	}
	if got := utf8.RuneCountInString(attempts[0].Username); got != maxLoginAttemptUsernameLength || !utf8.ValidString(attempts[0].Username) {
		t.Fatalf("username length: got %d runes", got)
	}
	if got := len(attempts[0].UserAgent); got != maxLoginAttemptUserAgentLength {
		t.Fatalf("user agent length: got %d", got)
	}
}

func TestDeleteOldLoginAttempts(t *testing.T) {
	st := openTestDB(t)
	ctx := context.Background()

	for _, username := range []string{"old", "recent"} {
		if err := st.InsertLoginAttempt(ctx, username, "10.0.0.1", "agent", "", "failed", ""); err != nil {
			t.Fatalf("insert %s: %v", username, err)
		}
	}
	old := time.Now().UTC().AddDate(0, -4, 0).Format("2006-01-02 15:04:05")
	if _, err := st.db.ExecContext(ctx, `UPDATE login_attempts SET attempted_at=? WHERE username='old'`, old); err != nil {
		t.Fatalf("backdate attempt: %v", err)
	}

	deleted, err := st.DeleteOldLoginAttempts(ctx, time.Now().AddDate(0, -3, 0))
	if err != nil || deleted != 1 {
		t.Fatalf("delete old attempts: deleted=%d err=%v", deleted, err)
	}
	attempts, err := st.ListLoginAttemptsPage(ctx, 10, 0)
	if err != nil || len(attempts) != 1 || attempts[0].Username != "recent" {
		t.Fatalf("remaining attempts: %v %+v", err, attempts)
	}
}
