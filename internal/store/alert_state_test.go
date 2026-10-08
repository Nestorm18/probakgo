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

func TestListPresentAlertsReturnsOnlyActiveStates(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	active := domain.Alert{
		ID:         "backup_error:pve:7:100",
		Type:       domain.AlertTypeBackupError,
		Severity:   domain.AlertSeverityCritical,
		Title:      "Backup fallido",
		Message:    "VM 100",
		ServerName: "pve-7",
		ServerType: "pve",
		ServerID:   7,
		VMID:       100,
		VMName:     "vm-100",
	}
	resolved := domain.Alert{
		ID:         "disk:windows:8:C:",
		Type:       domain.AlertTypeDisk,
		Severity:   domain.AlertSeverityWarning,
		Title:      "Disco casi lleno",
		ServerName: "windows-8",
		ServerType: "windows",
		ServerID:   8,
		StoreName:  "C:",
	}

	if err := st.SyncAlertStates(ctx, []domain.Alert{active, resolved}); err != nil {
		t.Fatalf("initial sync: %v", err)
	}
	if err := st.SyncAlertStates(ctx, []domain.Alert{active}); err != nil {
		t.Fatalf("resolve one alert: %v", err)
	}

	alerts, err := st.ListPresentAlerts(ctx)
	if err != nil {
		t.Fatalf("ListPresentAlerts: %v", err)
	}
	if len(alerts) != 1 {
		t.Fatalf("got %d active alerts, want 1: %+v", len(alerts), alerts)
	}
	got := alerts[0]
	if got.ID != active.ID || got.Type != domain.AlertTypeBackupError ||
		got.ServerID != active.ServerID || got.VMID != active.VMID || got.DetectedAt.IsZero() {
		t.Fatalf("unexpected active alert: %+v", got)
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

func TestDeleteOldAlertHistoryKeepsPendingAndPresentAlerts(t *testing.T) {
	st := openTestDB(t)
	ctx := context.Background()

	alert := func(id string) domain.Alert {
		return domain.Alert{ID: id, Severity: domain.AlertSeverityWarning, Title: id}
	}
	if err := st.SyncAlertStates(ctx, []domain.Alert{alert("resolved"), alert("pending"), alert("present")}); err != nil {
		t.Fatalf("seed alerts: %v", err)
	}
	if err := st.SyncAlertStates(ctx, []domain.Alert{alert("present")}); err != nil {
		t.Fatalf("resolve alerts: %v", err)
	}
	old := time.Now().AddDate(0, -6, 0)
	if _, err := st.db.ExecContext(ctx, `UPDATE alert_states SET updated_at=?`, old); err != nil {
		t.Fatalf("backdate states: %v", err)
	}
	if _, err := st.db.ExecContext(ctx, `UPDATE alert_states SET resolution_email_pending=1 WHERE alert_id='pending'`); err != nil {
		t.Fatalf("mark pending resolution: %v", err)
	}
	if _, err := st.db.ExecContext(ctx, `UPDATE alert_state_events SET created_at=?`, old.UTC().Format("2006-01-02 15:04:05")); err != nil {
		t.Fatalf("backdate events: %v", err)
	}
	if err := st.InsertAlertStateEvent(ctx, domain.AlertStateEvent{AlertID: "present", EventType: "suppressed"}); err != nil {
		t.Fatalf("insert recent event: %v", err)
	}

	events, states, err := st.DeleteOldAlertHistory(ctx, time.Now().AddDate(0, -3, 0))
	if err != nil {
		t.Fatalf("DeleteOldAlertHistory: %v", err)
	}
	if events == 0 || states != 1 {
		t.Fatalf("deleted events=%d states=%d, want old events and only the delivered resolved state", events, states)
	}
	assertCount(t, st, `SELECT COUNT(*) FROM alert_states WHERE alert_id = ?`, "resolved", 0)
	assertCount(t, st, `SELECT COUNT(*) FROM alert_states WHERE alert_id = ?`, "pending", 1)
	assertCount(t, st, `SELECT COUNT(*) FROM alert_states WHERE alert_id = ?`, "present", 1)
	assertCount(t, st, `SELECT COUNT(*) FROM alert_state_events WHERE event_type = ?`, "suppressed", 1)
}
