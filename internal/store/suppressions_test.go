package store

import (
	"context"
	"testing"
	"time"
)

func TestGetAlertSuppression(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)

	until := time.Now().Add(2 * time.Hour).Truncate(time.Second)
	if err := st.UpsertAlertSuppression(ctx, "a1", until, "maintenance"); err != nil {
		t.Fatalf("UpsertAlertSuppression: %v", err)
	}

	got, err := st.GetAlertSuppression(ctx, "a1")
	if err != nil {
		t.Fatalf("GetAlertSuppression: %v", err)
	}
	if got.AlertID != "a1" || got.Reason != "maintenance" || !got.Active {
		t.Fatalf("unexpected suppression: %+v", got)
	}
	if got.Until.Unix() != until.Unix() {
		t.Fatalf("until mismatch: got %v want %v", got.Until, until)
	}
}
