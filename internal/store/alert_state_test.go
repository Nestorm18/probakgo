package store

import (
	"context"
	"fmt"
	"testing"

	"probakgo/internal/domain"
)

func TestListAlertStateEventsPage(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)

	for i := 0; i < 30; i++ {
		if err := st.InsertAlertStateEvent(ctx, domain.AlertStateEvent{
			AlertID:    fmt.Sprintf("alert-%02d", i),
			EventType:  "appeared",
			Severity:   domain.AlertSeverityCritical,
			Title:      fmt.Sprintf("Alert %02d", i),
			ServerName: "pve",
			ServerType: "pve",
		}); err != nil {
			t.Fatalf("insert event %d: %v", i, err)
		}
	}

	events, err := st.ListAlertStateEventsPage(ctx, 10, 10)
	if err != nil {
		t.Fatalf("ListAlertStateEventsPage: %v", err)
	}
	if len(events) != 10 {
		t.Fatalf("want 10 events, got %d", len(events))
	}
}

func TestListAlertStateEventsForAlert(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)

	for _, id := range []string{"a1", "a2", "a1"} {
		if err := st.InsertAlertStateEvent(ctx, domain.AlertStateEvent{
			AlertID:    id,
			EventType:  "appeared",
			Severity:   domain.AlertSeverityWarning,
			Title:      id,
			ServerName: "pve",
			ServerType: "pve",
		}); err != nil {
			t.Fatalf("insert event %s: %v", id, err)
		}
	}

	events, err := st.ListAlertStateEventsForAlert(ctx, "a1", 10)
	if err != nil {
		t.Fatalf("ListAlertStateEventsForAlert: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("want 2 events, got %d", len(events))
	}
	for _, event := range events {
		if event.AlertID != "a1" {
			t.Fatalf("unexpected alert id %q", event.AlertID)
		}
	}
}
