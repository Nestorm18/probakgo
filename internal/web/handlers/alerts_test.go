package webhandlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	dbpkg "probakgo/internal/db"
	"probakgo/internal/domain"
	"probakgo/internal/store"
)

func openAlertsHandlerDB(t *testing.T) *store.Store {
	t.Helper()
	db, err := dbpkg.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return store.New(db)
}

func TestGroupAlertsByServer(t *testing.T) {
	alerts := []domain.Alert{
		{ID: "a1", ServerName: "soporte1", ServerType: "pve", ServerID: 1, Severity: domain.AlertSeverityWarning},
		{ID: "a2", ServerName: "soporte1", ServerType: "pve", ServerID: 1, Severity: domain.AlertSeverityCritical},
		{ID: "a3", ServerName: "pbs", ServerType: "pbs", ServerID: 1, Severity: domain.AlertSeverityWarning},
	}

	groups := groupAlertsByServer(alerts)

	if len(groups) != 2 {
		t.Fatalf("want 2 groups, got %d", len(groups))
	}
	if groups[0].ServerName != "soporte1" || groups[0].ServerType != "pve" {
		t.Fatalf("first group: got %+v, want soporte1/pve", groups[0])
	}
	if len(groups[0].Alerts) != 2 || groups[0].Critical != 1 || groups[0].Warning != 1 {
		t.Errorf("first group counts: got alerts=%d critical=%d warning=%d", len(groups[0].Alerts), groups[0].Critical, groups[0].Warning)
	}
	if groups[1].ServerName != "pbs" || len(groups[1].Alerts) != 1 || groups[1].Warning != 1 {
		t.Errorf("second group: got %+v", groups[1])
	}
}

func TestGroupSuppressedByServer(t *testing.T) {
	rows := []suppressedAlertRow{
		{Alert: domain.Alert{ID: "a1", ServerName: "soporte1", ServerType: "pve", ServerID: 1}, Until: time.Unix(100, 0)},
		{Alert: domain.Alert{ID: "a2", ServerName: "soporte1", ServerType: "pve", ServerID: 1}, Until: time.Unix(200, 0)},
		{Alert: domain.Alert{ID: "a3", ServerName: "pbs", ServerType: "pbs", ServerID: 1}, Until: time.Unix(300, 0)},
	}

	groups := groupSuppressedByServer(rows)

	if len(groups) != 2 {
		t.Fatalf("want 2 groups, got %d", len(groups))
	}
	if groups[0].ServerName != "soporte1" || len(groups[0].Rows) != 2 {
		t.Fatalf("first suppressed group: got %+v", groups[0])
	}
	if groups[1].ServerName != "pbs" || len(groups[1].Rows) != 1 {
		t.Fatalf("second suppressed group: got %+v", groups[1])
	}
}

func TestFormAlertIDs(t *testing.T) {
	req := httptest.NewRequest("POST", "/alerts/suppress", strings.NewReader("alert_id=a1,a2&alert_id=a2&alert_id=a3"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	got := formAlertIDs(req)

	want := []string{"a1", "a2", "a3"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestAlertRedirectBack(t *testing.T) {
	req := httptest.NewRequest("POST", "/alerts/suppress", strings.NewReader("back=/servers/windows/10"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if got := alertRedirectBack(req); got != "/servers/windows/10" {
		t.Fatalf("got %q", got)
	}
	req = httptest.NewRequest("POST", "/alerts/suppress", strings.NewReader("back=https://example.com"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if got := alertRedirectBack(req); got != "/alerts" {
		t.Fatalf("external redirect should fallback, got %q", got)
	}
}

func TestAlertsStatusIncludesHeartbeatAlert(t *testing.T) {
	st := openAlertsHandlerDB(t)
	ctx := context.Background()
	serverID, err := st.UpsertPVEServer(ctx, "pve-offline", "10.0.0.10", "", "0.0.71", "mid-1")
	if err != nil {
		t.Fatalf("UpsertPVEServer: %v", err)
	}
	if err := st.UpsertServerHeartbeat(ctx, domain.ServerHeartbeat{
		ServerType:    "pve",
		ServerID:      serverID,
		Hostname:      "pve-offline",
		ClientVersion: "0.0.71",
		MachineID:     "mid-1",
		LastSeenAt:    time.Now().Add(-30 * time.Minute),
	}); err != nil {
		t.Fatalf("UpsertServerHeartbeat: %v", err)
	}

	h := New(st, nil, nil)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/alerts/status.json", nil)
	h.AlertsStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, body %s", rr.Code, rr.Body.String())
	}
	var resp alertStatusResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Critical == 0 {
		t.Fatal("expected at least one critical alert")
	}
	for _, a := range resp.Alerts {
		if a.Type == domain.AlertTypePVEHeartbeat && a.ServerID == serverID && a.Title == "Servidor offline" {
			return
		}
	}
	t.Fatalf("heartbeat alert not found in response: %+v", resp.Alerts)
}
