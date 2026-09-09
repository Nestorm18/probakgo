package store

import (
	"context"
	"testing"
	"time"
)

func TestAlertEmailBatchWindow(t *testing.T) {
	st := openTestDB(t)
	ctx := context.Background()
	now := time.Unix(1800000000, 0)
	check := func(minutes int, pending bool, offset time.Duration, want bool) {
		t.Helper()
		got, err := st.AlertEmailBatchDue(ctx, minutes, pending, now.Add(offset))
		if err != nil || got != want {
			t.Fatalf("minutes=%d pending=%v offset=%v: got %v, %v; want %v", minutes, pending, offset, got, err, want)
		}
	}
	check(5, false, 0, false)
	check(5, true, 0, false)
	check(5, true, 4*time.Minute, false)
	check(5, true, 5*time.Minute, true)
	// An unsuccessful delivery leaves the batch due for retry.
	check(5, true, 6*time.Minute, true)
	if err := st.ClearAlertEmailBatch(ctx); err != nil {
		t.Fatal(err)
	}
	check(15, true, 7*time.Minute, false)
	check(15, true, 21*time.Minute, false)
	check(15, true, 22*time.Minute, true)
	check(15, false, 23*time.Minute, false)
	check(15, true, 24*time.Minute, false)
	check(0, true, 24*time.Minute, true)
}
