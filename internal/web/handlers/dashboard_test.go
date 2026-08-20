package webhandlers

import (
	"testing"
	"time"

	"probakgo/internal/domain"
)

func TestPBSFillBadge_IgnoresPastEstimateAndUsesPercent(t *testing.T) {
	label, class := pbsFillBadge([]domain.PBSStore{{
		Total:             2948636082176,
		Used:              2620000000000,
		EstimatedFullDate: time.Now().Add(-24 * time.Hour).Unix(),
	}})

	if label != "88% · Sin riesgo" || class != "ok" {
		t.Fatalf("want 88%% · Sin riesgo ok, got %s %s", label, class)
	}
}

func TestPBSFillBadge_FutureEstimateWins(t *testing.T) {
	label, class := pbsFillBadge([]domain.PBSStore{{
		Total:             2948636082176,
		Used:              2620000000000,
		EstimatedFullDate: time.Now().Add(10 * 24 * time.Hour).Unix(),
	}})

	if label != "Lleno en 10d" || class != "bad" {
		t.Fatalf("want Lleno en 10d bad, got %s %s", label, class)
	}
}

func TestPBSStoreDisplays_UsesSameFillStatus(t *testing.T) {
	rows := pbsStoreDisplays([]domain.PBSStore{{
		Store:             "synology",
		Total:             2948636082176,
		Used:              2620000000000,
		EstimatedFullDate: time.Now().Add(-24 * time.Hour).Unix(),
	}})

	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	if rows[0].StoreName != "synology" || rows[0].BadgeLabel != "88%" || rows[0].BadgeClass != "ok" || !rows[0].NoFillRisk {
		t.Fatalf("unexpected row: %+v", rows[0])
	}
}

func TestPBSMaxStoreUsagePercent(t *testing.T) {
	stores := []domain.PBSStore{
		{Used: 20, Total: 100},
		{Used: 75, Total: 100},
		{Used: 10, Total: 0},
	}
	if got := pbsMaxStoreUsagePercent(stores); got != 75 {
		t.Fatalf("pbsMaxStoreUsagePercent() = %d, want 75", got)
	}
	if got := pbsMaxStoreUsagePercent(nil); got != -1 {
		t.Fatalf("pbsMaxStoreUsagePercent(nil) = %d, want -1", got)
	}
}

func TestSummarizeDashboardAlertsFiltersAndPrioritizes(t *testing.T) {
	alerts := []domain.Alert{
		{ID: "backup_error:pve:1:100", Type: domain.AlertTypeBackupError, Severity: domain.AlertSeverityWarning, ServerType: "pve", ServerID: 1},
		{ID: "backup_error:pve:1:101", Type: domain.AlertTypeBackupError, Severity: domain.AlertSeverityCritical, ServerType: "pve", ServerID: 1},
		{ID: "windows_volume_missing:windows:2:D:", Type: domain.AlertTypeWindowsVolumeGone, Severity: domain.AlertSeverityCritical, ServerType: "windows", ServerID: 2},
		{ID: "disk:windows:3:C:", Type: domain.AlertTypeDisk, Severity: domain.AlertSeverityCritical, ServerType: "windows", ServerID: 3},
		{ID: "disk:pbs:4:store", Type: domain.AlertTypeDisk, Severity: domain.AlertSeverityWarning, ServerType: "pbs", ServerID: 4},
	}
	suppressed := map[string]time.Time{
		"disk:windows:3:C:": time.Now().Add(time.Hour),
	}
	maintenance := map[string]domain.ServerMaintenance{
		"pbs:4": {ServerType: "pbs", ServerID: 4, Active: true, Until: time.Now().Add(time.Hour)},
	}

	got := summarizeDashboardAlerts(alerts, suppressed, maintenance)

	if got.Critical != 2 || got.Warning != 1 {
		t.Fatalf("counts: got critical=%d warning=%d, want 2/1", got.Critical, got.Warning)
	}
	if got.PVEBackupSeverity[1] != domain.AlertSeverityCritical {
		t.Fatalf("PVE backup severity: got %q, want critical", got.PVEBackupSeverity[1])
	}
	if !got.WindowsMissingVolume[2] {
		t.Fatal("missing Windows volume alert was not retained")
	}
}

func TestDashboardStatusURL(t *testing.T) {
	tests := []struct {
		name       string
		serverType string
		serverIDs  []int64
		want       string
	}{
		{name: "none", serverType: "pve", want: ""},
		{name: "single", serverType: "pve", serverIDs: []int64{7}, want: "/servers/pve/7"},
		{name: "multiple", serverType: "pbs", serverIDs: []int64{3, 9}, want: "/servers/pbs?filter=Sin+reporte"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dashboardStatusURL(tt.serverType, tt.serverIDs, "Sin reporte"); got != tt.want {
				t.Fatalf("dashboardStatusURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDashboardMaintenanceURL(t *testing.T) {
	if got := dashboardMaintenanceURL(nil); got != "" {
		t.Fatalf("empty maintenance URL = %q, want empty", got)
	}

	single := map[string]domain.ServerMaintenance{
		"pve:7": {ServerType: "pve", ServerID: 7, Active: true},
	}
	if got := dashboardMaintenanceURL(single); got != "/servers/pve/7" {
		t.Fatalf("single maintenance URL = %q, want /servers/pve/7", got)
	}

	multiple := map[string]domain.ServerMaintenance{
		"pve:7":     {ServerType: "pve", ServerID: 7, Active: true},
		"windows:4": {ServerType: "windows", ServerID: 4, Active: true},
	}
	if got := dashboardMaintenanceURL(multiple); got != "/?filter=Mantenimiento#dashboard-servers" {
		t.Fatalf("multiple maintenance URL = %q", got)
	}
}
