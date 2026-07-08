package store

import (
	"context"
	"fmt"
	"testing"
	"time"

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

func TestAlertCriticalEmailSentResetsWhenResolved(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	alert := domain.Alert{
		ID:         "pve_heartbeat:pve:1",
		Type:       domain.AlertTypePVEHeartbeat,
		Severity:   domain.AlertSeverityCritical,
		Title:      "Servidor offline",
		ServerName: "pve-1",
		ServerType: "pve",
		ServerID:   1,
	}

	if err := st.SyncAlertStates(ctx, []domain.Alert{alert}); err != nil {
		t.Fatalf("sync alert: %v", err)
	}
	_, sent, err := st.GetAlertCriticalEmailSentAt(ctx, alert.ID)
	if err != nil {
		t.Fatalf("get initial sent state: %v", err)
	}
	if sent {
		t.Fatal("new alert should not be marked as emailed")
	}

	if err := st.MarkAlertCriticalEmailSent(ctx, alert.ID, time.Now()); err != nil {
		t.Fatalf("mark sent: %v", err)
	}
	_, sent, err = st.GetAlertCriticalEmailSentAt(ctx, alert.ID)
	if err != nil {
		t.Fatalf("get sent state: %v", err)
	}
	if !sent {
		t.Fatal("alert should be marked as emailed")
	}

	if err := st.SyncAlertStates(ctx, nil); err != nil {
		t.Fatalf("resolve alert: %v", err)
	}
	_, sent, err = st.GetAlertCriticalEmailSentAt(ctx, alert.ID)
	if err != nil {
		t.Fatalf("get resolved sent state: %v", err)
	}
	if sent {
		t.Fatal("resolved alert should not be treated as emailed")
	}
	pending, err := st.ListPendingAlertResolutionEmails(ctx)
	if err != nil {
		t.Fatalf("list pending resolution emails: %v", err)
	}
	if len(pending) != 1 || pending[0].ID != alert.ID {
		t.Fatalf("pending resolution emails: got %+v, want alert %s", pending, alert.ID)
	}

	if err := st.MarkAlertResolutionEmailSent(ctx, alert.ID, time.Now()); err != nil {
		t.Fatalf("mark resolution sent: %v", err)
	}
	pending, err = st.ListPendingAlertResolutionEmails(ctx)
	if err != nil {
		t.Fatalf("list pending after mark: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("pending after mark: got %d, want 0", len(pending))
	}

	if err := st.SyncAlertStates(ctx, []domain.Alert{alert}); err != nil {
		t.Fatalf("reappear alert: %v", err)
	}
	_, sent, err = st.GetAlertCriticalEmailSentAt(ctx, alert.ID)
	if err != nil {
		t.Fatalf("get reappeared sent state: %v", err)
	}
	if sent {
		t.Fatal("reappeared alert should be eligible for a new email")
	}
	pending, err = st.ListPendingAlertResolutionEmails(ctx)
	if err != nil {
		t.Fatalf("list pending after reappear: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("reappeared alert should not keep old resolution email pending")
	}
}
