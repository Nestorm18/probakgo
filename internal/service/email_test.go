package service

import (
	"context"
	"testing"
	"time"

	"probakgo/internal/domain"
)

func TestParseRecipients_Multiple(t *testing.T) {
	got := parseRecipients("a@b.com, c@d.com, e@f.com")
	if len(got) != 3 {
		t.Fatalf("want 3 recipients, got %d: %v", len(got), got)
	}
	if got[0] != "a@b.com" || got[1] != "c@d.com" || got[2] != "e@f.com" {
		t.Errorf("unexpected recipients: %v", got)
	}
}

func TestParseRecipients_Empty(t *testing.T) {
	got := parseRecipients("")
	if len(got) != 0 {
		t.Errorf("want empty slice, got %v", got)
	}
}

func TestNextRunTime_Future(t *testing.T) {
	// 23:59 is in the future for all but 1 minute per day
	next := nextRunTime("23:59", time.UTC)
	if !next.After(time.Now()) {
		t.Error("want future time")
	}
	if next.Hour() != 23 || next.Minute() != 59 {
		t.Errorf("want time at 23:59, got %02d:%02d", next.Hour(), next.Minute())
	}
}

func TestNextRunTime_Past(t *testing.T) {
	// 00:01 has almost certainly passed by UTC today
	next := nextRunTime("00:01", time.UTC)
	if !next.After(time.Now()) {
		t.Error("want future time even for a past send time")
	}
	if next.Hour() != 0 || next.Minute() != 1 {
		t.Errorf("want time at 00:01, got %02d:%02d", next.Hour(), next.Minute())
	}
}

func TestBuildEmailData_AllOK(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	svc := NewReport(st, time.UTC)

	serverID, _ := st.UpsertPVEServer(ctx, "pve-ok", "10.0.0.1", "", "1.0", "")
	_, _ = st.InsertPVEReport(ctx, serverID, nil) // reported now → not stale

	cfg, _ := st.GetEmailConfig(ctx)
	// Disable disk and backup-error alerts so only staleness matters
	cfg.AlertDiskPct = 0
	cfg.AlertBackupErr = false

	data, err := buildEmailData(ctx, st, svc, cfg)
	if err != nil {
		t.Fatalf("buildEmailData: %v", err)
	}
	if data.TotalIssues != 0 {
		t.Errorf("want TotalIssues=0, got %d", data.TotalIssues)
	}
	if data.HeaderColor != "#28a745" {
		t.Errorf("want green header, got %q", data.HeaderColor)
	}
	if len(data.PVEOk) != 1 {
		t.Errorf("want 1 in PVEOk, got %d", len(data.PVEOk))
	}
	if len(data.PVEIssues) != 0 {
		t.Errorf("want 0 in PVEIssues, got %d", len(data.PVEIssues))
	}
}

func TestBuildEmailData_WithStale(t *testing.T) {
	ctx := context.Background()
	db, st := openTestStore(t)
	svc := NewReport(st, time.UTC)

	serverID, _ := st.UpsertPVEServer(ctx, "pve-stale", "10.0.0.1", "", "1.0", "")
	oldID, _ := st.InsertPVEReport(ctx, serverID, nil)
	yesterday := time.Now().Add(-25 * time.Hour)
	db.Exec("UPDATE pve_reports SET reported_at = ? WHERE id = ?", yesterday, oldID)

	cfg, _ := st.GetEmailConfig(ctx)
	cfg.AlertDiskPct = 0
	cfg.AlertBackupErr = false

	data, err := buildEmailData(ctx, st, svc, cfg)
	if err != nil {
		t.Fatalf("buildEmailData: %v", err)
	}
	if len(data.PVEIssues) != 1 {
		t.Fatalf("want 1 in PVEIssues, got %d", len(data.PVEIssues))
	}
	if data.PVEIssues[0].Name != "pve-stale" {
		t.Errorf("want pve-stale in issues, got %q", data.PVEIssues[0].Name)
	}
	if data.TotalIssues == 0 {
		t.Error("want TotalIssues > 0")
	}
	if data.HeaderColor != "#dc3545" {
		t.Errorf("want red header, got %q", data.HeaderColor)
	}
}

func TestBuildEmailData_StaleReportDoesNotShowOldTasksOK(t *testing.T) {
	ctx := context.Background()
	db, st := openTestStore(t)
	svc := NewReport(st, time.UTC)

	serverID, _ := st.UpsertPVEServer(ctx, "pve-stale", "10.0.0.1", "", "1.0", "")
	oldID, _ := st.InsertPVEReport(ctx, serverID, nil)
	if err := st.InsertPVEBackupTask(ctx, oldID, domain.BackupTaskPayload{
		VMID:      101,
		VMName:    "mikrotik-routeros-chr",
		Status:    "OK",
		StartTime: time.Now().Add(-26 * time.Hour).Unix(),
		EndTime:   time.Now().Add(-26*time.Hour + 175*time.Second).Unix(),
		Duration:  175,
		Size:      41_860_000,
	}); err != nil {
		t.Fatalf("InsertPVEBackupTask: %v", err)
	}
	yesterday := time.Now().Add(-25 * time.Hour)
	db.Exec("UPDATE pve_reports SET reported_at = ? WHERE id = ?", yesterday, oldID)

	cfg, _ := st.GetEmailConfig(ctx)
	cfg.AlertDiskPct = 0
	cfg.AlertBackupErr = false

	data, err := buildEmailData(ctx, st, svc, cfg)
	if err != nil {
		t.Fatalf("buildEmailData: %v", err)
	}
	if len(data.PVEIssues) != 1 {
		t.Fatalf("want 1 in PVEIssues, got %d", len(data.PVEIssues))
	}
	tasks := data.PVEIssues[0].VMTasks
	if len(tasks) != 1 {
		t.Fatalf("want 1 VM task, got %d", len(tasks))
	}
	if !tasks[0].IsMissing {
		t.Fatal("want stale old task to be shown as missing")
	}
	if tasks[0].Status == "OK" {
		t.Fatal("stale old task must not be shown as OK")
	}
}

func TestBuildEmailData_DiskAlertsDoNotCountAsBackupProblems(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	svc := NewReport(st, time.UTC)

	serverID, _ := st.UpsertPVEServer(ctx, "pve-disk-alert", "10.0.0.1", "", "1.0", "")
	reportID, _ := st.InsertPVEReport(ctx, serverID, nil)
	storageID, _ := st.InsertPVEStorage(ctx, reportID, domain.StoragePayload{
		Storage: "backup-store",
		Content: "backup",
	})
	_ = st.InsertPVEStorageInfo(ctx, storageID, domain.StorageInfoPayload{
		Total:   1000,
		Used:    900,
		Avail:   100,
		Active:  true,
		Enabled: true,
	})

	cfg, _ := st.GetEmailConfig(ctx)
	cfg.AlertDiskPct = 85
	cfg.AlertBackupErr = false
	if err := st.UpsertEmailConfig(ctx, *cfg); err != nil {
		t.Fatalf("UpsertEmailConfig: %v", err)
	}

	data, err := buildEmailData(ctx, st, svc, cfg)
	if err != nil {
		t.Fatalf("buildEmailData: %v", err)
	}
	if len(data.DiskAlerts) != 1 {
		t.Fatalf("want 1 disk alert, got %d", len(data.DiskAlerts))
	}
	if data.TotalIssues != 0 {
		t.Errorf("want TotalIssues=0 for disk-only alert, got %d", data.TotalIssues)
	}
	if data.HeaderColor != "#28a745" {
		t.Errorf("want green header for disk-only alert, got %q", data.HeaderColor)
	}
	if data.StatusText != "Todos los servidores operativos" {
		t.Errorf("unexpected status text: %q", data.StatusText)
	}
}

func TestBuildEmailData_MissingActiveVMsMakePVEIssue(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	svc := NewReport(st, time.UTC)

	serverID, _ := st.UpsertPVEServer(ctx, "soporte2", "192.168.10.250", "", "0.0.44", "")
	reportID, _ := st.InsertPVEReport(ctx, serverID, nil)
	now := time.Now()
	if err := st.InsertPVEBackupTask(ctx, reportID, domain.BackupTaskPayload{
		VMID:      100,
		VMName:    "adguard",
		Status:    "OK",
		StartTime: now.Unix(),
		EndTime:   now.Add(40 * time.Second).Unix(),
		Duration:  40,
		Size:      783_350_000,
	}); err != nil {
		t.Fatalf("InsertPVEBackupTask: %v", err)
	}
	for _, vm := range []struct {
		id       string
		name     string
		excluded bool
	}{
		{"100", "adguard", false},
		{"300", "wireguard", false},
		{"301", "wireguard-externos", false},
		{"1000", "pbs-example", true},
	} {
		_, err := st.CreateVMBackupConfig(ctx, "soporte2", domain.CreateVMBackupConfigRequest{
			VMID:      vm.id,
			VMName:    vm.name,
			Monday:    true,
			Tuesday:   true,
			Wednesday: true,
			Thursday:  true,
			Friday:    true,
			Saturday:  true,
			Sunday:    true,
		})
		if err != nil {
			t.Fatalf("CreateVMBackupConfig %s: %v", vm.id, err)
		}
		if vm.excluded {
			if err := st.ToggleVMExclude(ctx, "soporte2", vm.id); err != nil {
				t.Fatalf("ToggleVMExclude %s: %v", vm.id, err)
			}
		}
	}

	cfg, _ := st.GetEmailConfig(ctx)
	cfg.AlertDiskPct = 0
	cfg.AlertBackupErr = false

	data, err := buildEmailData(ctx, st, svc, cfg)
	if err != nil {
		t.Fatalf("buildEmailData: %v", err)
	}
	if len(data.PVEIssues) != 1 {
		t.Fatalf("want 1 PVE issue, got %d", len(data.PVEIssues))
	}
	if data.PVEIssues[0].Name != "soporte2" {
		t.Errorf("issue server: got %q, want soporte2", data.PVEIssues[0].Name)
	}
	if data.TotalIssues != 1 {
		t.Errorf("TotalIssues: got %d, want 1", data.TotalIssues)
	}
	missing := make(map[string]vmTaskRow)
	for _, task := range data.PVEIssues[0].VMTasks {
		if task.IsMissing {
			missing[task.VMID] = task
		}
	}
	if !missing["300"].IsMissing || missing["300"].IsExcluded {
		t.Errorf("VM 300 should be active missing, got %+v", missing["300"])
	}
	if !missing["301"].IsMissing || missing["301"].IsExcluded {
		t.Errorf("VM 301 should be active missing, got %+v", missing["301"])
	}
	if !missing["1000"].IsMissing || !missing["1000"].IsExcluded {
		t.Errorf("VM 1000 should be excluded missing warning, got %+v", missing["1000"])
	}
}
