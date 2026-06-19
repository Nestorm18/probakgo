package store

import (
	"context"
	"testing"
)

func TestListLoginAttemptsPage(t *testing.T) {
	st := openTestDB(t)
	ctx := context.Background()

	for _, username := range []string{"first", "second", "third"} {
		if err := st.InsertLoginAttempt(ctx, username, "10.0.0.1", "agent", "failed", "bad password"); err != nil {
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
}
