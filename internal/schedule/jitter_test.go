package schedule

import "testing"

func TestDailyMinuteIsStableAndBounded(t *testing.T) {
	first := DailyMinute("machine-a:server")
	if first < 0 || first > 59 {
		t.Fatalf("DailyMinute = %d, want 0..59", first)
	}
	if got := DailyMinute("machine-a:server"); got != first {
		t.Fatalf("DailyMinute changed: first=%d second=%d", first, got)
	}
	if got := DailyMinute("machine-b:server"); got == first {
		t.Fatalf("test seeds unexpectedly collided at minute %d", first)
	}
}
