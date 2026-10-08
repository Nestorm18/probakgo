package store

import (
	"context"
	"testing"
	"time"
)

func TestRevokeSessionListsOnlyThatSessionAndPurgesExpired(t *testing.T) {
	st := openTestDB(t)
	ctx := context.Background()

	if err := st.RevokeSession(ctx, "expired-sid", time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("RevokeSession expired: %v", err)
	}
	if err := st.RevokeSession(ctx, "closed-sid", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("RevokeSession: %v", err)
	}

	for sid, want := range map[string]bool{"closed-sid": true, "other-sid": false, "expired-sid": false} {
		got, err := st.IsSessionRevoked(ctx, sid)
		if err != nil {
			t.Fatalf("IsSessionRevoked(%q): %v", sid, err)
		}
		if got != want {
			t.Errorf("IsSessionRevoked(%q) = %v, want %v", sid, got, want)
		}
	}

	var raw int
	if err := st.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM revoked_sessions WHERE sid_hash IN ('closed-sid', 'expired-sid')`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if raw != 0 {
		t.Fatal("raw session IDs were stored instead of their hashes")
	}
}
