package webhandlers

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"probakgo/internal/domain"
)

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
