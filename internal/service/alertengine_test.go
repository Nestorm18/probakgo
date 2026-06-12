package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"probakgo/internal/domain"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func defaultCfg() AlertConfigs {
	return AlertConfigs{
		GlobalDiskPct:             85,
		GlobalBackupErr:           true,
		GlobalStaleHours:          48,
		GlobalPVEHeartbeatMinutes: 15,
		PVEConfigs:                make(map[int64]domain.PVEAlertConfig),
		PVEVMConfigs:              make(map[int64][]domain.PVEVMAlertConfig),
		PBSConfigs:                make(map[int64]domain.PBSAlertConfig),
	}
}

func hasAlert(alerts []domain.Alert, typ, serverName string) bool {
	for _, a := range alerts {
		if a.Type == typ && a.ServerName == serverName {
			return true
		}
	}
	return false
}

func hasAlertForVM(alerts []domain.Alert, typ string, vmid int64) bool {
	for _, a := range alerts {
		if a.Type == typ && a.VMID == vmid {
			return true
		}
	}
	return false
}

// ── evalPVEDisk ───────────────────────────────────────────────────────────────

func TestEvalPVEDisk_OverThreshold(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	serverID, _ := st.UpsertPVEServer(ctx, "pve1", "1.1.1.1", "", "1.0", "")

	reportID, _ := st.InsertPVEReport(ctx, serverID, nil)
	stgID, _ := st.InsertPVEStorage(ctx, reportID, domain.StoragePayload{Storage: "backup-store", Content: "backup"})
	_ = st.InsertPVEStorageInfo(ctx, stgID, domain.StorageInfoPayload{Total: 1000, Used: 900, Avail: 100, Active: true, Enabled: true})

	cfg := defaultCfg()
	alerts, err := evalPVEDisk(st, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasAlert(alerts, domain.AlertTypeDisk, "pve1") {
		t.Error("expected disk alert for pve1, got none")
	}
}

func TestEvalPVEDisk_UnderThreshold(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	serverID, _ := st.UpsertPVEServer(ctx, "pve2", "1.1.1.1", "", "1.0", "")
	reportID, _ := st.InsertPVEReport(ctx, serverID, nil)
	stgID, _ := st.InsertPVEStorage(ctx, reportID, domain.StoragePayload{Storage: "backup-store", Content: "backup"})
	_ = st.InsertPVEStorageInfo(ctx, stgID, domain.StorageInfoPayload{Total: 1000, Used: 500, Avail: 500, Active: true, Enabled: true})

	cfg := defaultCfg()
	alerts, _ := evalPVEDisk(st, cfg)
	if hasAlert(alerts, domain.AlertTypeDisk, "pve2") {
		t.Error("unexpected disk alert for pve2 at 50% usage")
	}
}

func TestEvalPVEDisk_PerServerThreshold(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	serverID, _ := st.UpsertPVEServer(ctx, "pve3", "1.1.1.1", "", "1.0", "")
	reportID, _ := st.InsertPVEReport(ctx, serverID, nil)
	stgID, _ := st.InsertPVEStorage(ctx, reportID, domain.StoragePayload{Storage: "backup-store", Content: "backup"})
	_ = st.InsertPVEStorageInfo(ctx, stgID, domain.StorageInfoPayload{Total: 1000, Used: 700, Avail: 300, Active: true, Enabled: true})

	cfg := defaultCfg() // global=85%, usage=70% → no alert normally
	threshold := 60
	cfg.PVEConfigs[serverID] = domain.PVEAlertConfig{ServerID: serverID, DiskPct: &threshold}

	alerts, _ := evalPVEDisk(st, cfg)
	if !hasAlert(alerts, domain.AlertTypeDisk, "pve3") {
		t.Error("expected disk alert with per-server threshold=60, usage=70%")
	}
}

func TestEvalPVEHeartbeat_Stale(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	serverID, _ := st.UpsertPVEServer(ctx, "pve-heartbeat", "1.1.1.1", "", "1.0", "mid-1")
	_ = st.UpsertServerHeartbeat(ctx, domain.ServerHeartbeat{
		ServerType: "pve",
		ServerID:   serverID,
		Hostname:   "pve-heartbeat",
		LastSeenAt: time.Now().Add(-20 * time.Minute),
	})

	cfg := defaultCfg()
	cfg.GlobalPVEHeartbeatMinutes = 15
	alerts, err := evalPVEHeartbeat(st, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasAlert(alerts, domain.AlertTypePVEHeartbeat, "pve-heartbeat") {
		t.Fatal("expected heartbeat alert")
	}
}

func TestEvalPVEHeartbeat_NoAlertBeforeFirstHeartbeat(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	_, _ = st.UpsertPVEServer(ctx, "pve-no-heartbeat-yet", "1.1.1.1", "", "1.0", "")

	cfg := defaultCfg()
	cfg.GlobalPVEHeartbeatMinutes = 15
	alerts, err := evalPVEHeartbeat(st, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasAlert(alerts, domain.AlertTypePVEHeartbeat, "pve-no-heartbeat-yet") {
		t.Fatal("did not expect alert before first heartbeat")
	}
}

// ── evalPVEBackupErrors ───────────────────────────────────────────────────────

func TestEvalPVEBackupErrors_ErrorTask(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	serverID, _ := st.UpsertPVEServer(ctx, "pve-err", "1.1.1.1", "", "1.0", "")
	bs := &domain.BackupStatus{Status: json.RawMessage(`"PARTIAL"`)}
	reportID, _ := st.InsertPVEReport(ctx, serverID, bs)
	_ = st.InsertPVEBackupTask(ctx, reportID, domain.BackupTaskPayload{VMID: 101, VMName: "debian", Status: "PARTIAL"})

	cfg := defaultCfg()
	alerts, _ := evalPVEBackupErrors(st, cfg)
	if !hasAlertForVM(alerts, domain.AlertTypeBackupError, 101) {
		t.Error("expected backup_error alert for VM 101")
	}
}

func TestEvalPVEBackupErrors_OKTask_NoAlert(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	serverID, _ := st.UpsertPVEServer(ctx, "pve-ok", "1.1.1.1", "", "1.0", "")
	bs := &domain.BackupStatus{Status: json.RawMessage(`"OK"`)}
	reportID, _ := st.InsertPVEReport(ctx, serverID, bs)
	_ = st.InsertPVEBackupTask(ctx, reportID, domain.BackupTaskPayload{VMID: 101, VMName: "debian", Status: "OK"})

	cfg := defaultCfg()
	alerts, _ := evalPVEBackupErrors(st, cfg)
	if hasAlertForVM(alerts, domain.AlertTypeBackupError, 101) {
		t.Error("unexpected backup_error alert for OK task")
	}
}

func TestEvalPVEBackupErrors_ReportStatusFallbackForOldClients(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	serverID, _ := st.UpsertPVEServer(ctx, "pve-old-client", "1.1.1.1", "", "0.0.60", "")
	bs := &domain.BackupStatus{Status: json.RawMessage(`false`)}
	_, _ = st.InsertPVEReport(ctx, serverID, bs)

	cfg := defaultCfg()
	alerts, _ := evalPVEBackupErrors(st, cfg)

	if !hasAlert(alerts, domain.AlertTypeBackupError, "pve-old-client") {
		t.Error("expected backup_error alert from report backup_status when no tasks exist")
	}
}

func TestEvalPVEBackupErrors_VMOverride_Ignore(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	serverID, _ := st.UpsertPVEServer(ctx, "pve-override", "1.1.1.1", "", "1.0", "")
	reportID, _ := st.InsertPVEReport(ctx, serverID, nil)
	_ = st.InsertPVEBackupTask(ctx, reportID, domain.BackupTaskPayload{VMID: 102, VMName: "win", Status: "ERROR"})

	cfg := defaultCfg() // global backup_err=true
	ignore := 0
	cfg.PVEVMConfigs[serverID] = []domain.PVEVMAlertConfig{
		{ServerID: serverID, VMID: 102, BackupErr: &ignore}, // VM override: ignore
	}
	alerts, _ := evalPVEBackupErrors(st, cfg)
	if hasAlertForVM(alerts, domain.AlertTypeBackupError, 102) {
		t.Error("expected no backup_error alert: VM override set to ignore")
	}
}

func TestEvalPVEBackupErrors_ServerDisable_VMOverrideAlert(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	serverID, _ := st.UpsertPVEServer(ctx, "pve-mixed", "1.1.1.1", "", "1.0", "")
	reportID, _ := st.InsertPVEReport(ctx, serverID, nil)
	_ = st.InsertPVEBackupTask(ctx, reportID, domain.BackupTaskPayload{VMID: 103, VMName: "db", Status: "ERROR"})

	cfg := defaultCfg()
	serverIgnore := 0
	vmAlert := 1
	cfg.PVEConfigs[serverID] = domain.PVEAlertConfig{ServerID: serverID, BackupErr: &serverIgnore} // server=ignore
	cfg.PVEVMConfigs[serverID] = []domain.PVEVMAlertConfig{
		{ServerID: serverID, VMID: 103, BackupErr: &vmAlert}, // VM override: alert
	}
	alerts, _ := evalPVEBackupErrors(st, cfg)
	if !hasAlertForVM(alerts, domain.AlertTypeBackupError, 103) {
		t.Error("expected backup_error alert: VM override=alert even though server=ignore")
	}
}

// ── evalPVEBackupSize ─────────────────────────────────────────────────────────

func TestEvalPVEBackupSize_TooSmall(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	serverID, _ := st.UpsertPVEServer(ctx, "pve-size", "1.1.1.1", "", "1.0", "")
	reportID, _ := st.InsertPVEReport(ctx, serverID, nil)
	// backup size = 10 MB, min = 500 MB
	_ = st.InsertPVEBackupTask(ctx, reportID, domain.BackupTaskPayload{VMID: 200, VMName: "tiny", Status: "OK", Size: 10 * 1024 * 1024})

	cfg := defaultCfg()
	minMB := 500
	cfg.PVEVMConfigs[serverID] = []domain.PVEVMAlertConfig{
		{ServerID: serverID, VMID: 200, MinSizeMB: &minMB},
	}
	alerts, _ := evalPVEBackupSize(st, cfg)
	if !hasAlertForVM(alerts, domain.AlertTypeBackupSize, 200) {
		t.Error("expected backup_size alert for 10 MB backup with 500 MB min")
	}
}

func TestEvalPVEBackupSize_BigEnough_NoAlert(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	serverID, _ := st.UpsertPVEServer(ctx, "pve-bigsize", "1.1.1.1", "", "1.0", "")
	reportID, _ := st.InsertPVEReport(ctx, serverID, nil)
	_ = st.InsertPVEBackupTask(ctx, reportID, domain.BackupTaskPayload{VMID: 201, VMName: "big", Status: "OK", Size: 600 * 1024 * 1024})

	cfg := defaultCfg()
	minMB := 500
	cfg.PVEVMConfigs[serverID] = []domain.PVEVMAlertConfig{
		{ServerID: serverID, VMID: 201, MinSizeMB: &minMB},
	}
	alerts, _ := evalPVEBackupSize(st, cfg)
	if hasAlertForVM(alerts, domain.AlertTypeBackupSize, 201) {
		t.Error("unexpected backup_size alert for 600 MB backup with 500 MB min")
	}
}

// ── evalPVEStale ──────────────────────────────────────────────────────────────

func TestEvalPVEStale_StaleServer(t *testing.T) {
	ctx := context.Background()
	db, st := openTestStore(t)
	serverID, _ := st.UpsertPVEServer(ctx, "pve-stale", "1.1.1.1", "", "1.0", "")
	reportID, _ := st.InsertPVEReport(ctx, serverID, nil)
	// backdate report so is_stale=1 gets set
	_, _ = db.Exec(`UPDATE pve_reports SET is_stale=1, stale_reason='No se ha recibido el reporte de hoy' WHERE id=?`, reportID)

	cfg := defaultCfg()
	alerts, _ := evalPVEStale(st, cfg)
	if !hasAlert(alerts, domain.AlertTypePVEStale, "pve-stale") {
		t.Error("expected pve_stale alert for stale server")
	}
}

func TestEvalPVEStale_NotStale_NoAlert(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	serverID, _ := st.UpsertPVEServer(ctx, "pve-fresh", "1.1.1.1", "", "1.0", "")
	_, _ = st.InsertPVEReport(ctx, serverID, nil)

	cfg := defaultCfg()
	alerts, _ := evalPVEStale(st, cfg)
	if hasAlert(alerts, domain.AlertTypePVEStale, "pve-fresh") {
		t.Error("unexpected pve_stale alert for non-stale server")
	}
}

func TestEvalPVEStale_NoReport(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	_, _ = st.UpsertPVEServer(ctx, "pve-never", "1.1.1.1", "", "1.0", "")

	cfg := defaultCfg()
	cfg.Report = NewReport(st, time.UTC)
	alerts, _ := evalPVEStale(st, cfg)
	if !hasAlert(alerts, domain.AlertTypePVEStale, "pve-never") {
		t.Error("expected pve_stale alert for server without reports")
	}
}

// ── evalPBSDisk ───────────────────────────────────────────────────────────────

func TestEvalPBSDisk_OverThreshold(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	serverID, _ := st.UpsertPBSServer(ctx, "pbs1", "1.1.1.1", "", "1.0", "")
	reportID, _ := st.InsertPBSReport(ctx, serverID)
	_, _ = st.InsertPBSStore(ctx, reportID, domain.PBSDatastorePayload{
		Store: "datastore1", Total: 1000, Used: 900,
		EstimatedFullDate: time.Now().Add(10 * 24 * time.Hour).Unix(),
	})

	cfg := defaultCfg()
	alerts, _ := evalPBSDisk(st, cfg)
	if !hasAlert(alerts, domain.AlertTypeDisk, "pbs1") {
		t.Error("expected disk alert for pbs1")
	}
	if len(alerts) != 1 || !strings.Contains(alerts[0].Message, "estimacion: 9 dias") {
		t.Fatalf("expected disk alert to include fill estimation, got %+v", alerts)
	}
}

func TestEvalPBSReportStale_NoReport(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	_, _ = st.UpsertPBSServer(ctx, "pbs-never", "1.1.1.1", "", "1.0", "")

	cfg := defaultCfg()
	cfg.Report = NewReport(st, time.UTC)
	alerts, _ := evalPBSReportStale(st, cfg)
	if !hasAlert(alerts, domain.AlertTypePBSReportStale, "pbs-never") {
		t.Error("expected pbs_report_stale alert for server without reports")
	}
}

// ── evalPBSFill ───────────────────────────────────────────────────────────────

func TestEvalPBSFill_WithinThreshold(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	serverID, _ := st.UpsertPBSServer(ctx, "pbs-fill", "1.1.1.1", "", "1.0", "")
	reportID, _ := st.InsertPBSReport(ctx, serverID)
	fillDate := time.Now().Add(10 * 24 * time.Hour).Unix() // fills in 10 days
	_, _ = st.InsertPBSStore(ctx, reportID, domain.PBSDatastorePayload{
		Store: "ds1", Total: 1000, Used: 500, EstimatedFullDate: fillDate,
	})

	cfg := defaultCfg()
	threshold := 30
	cfg.PBSConfigs[serverID] = domain.PBSAlertConfig{ServerID: serverID, DaysUntilFull: &threshold, VerifyAlert: true}

	alerts, _ := evalPBSFill(st, cfg)
	if !hasAlert(alerts, domain.AlertTypePBSFill, "pbs-fill") {
		t.Error("expected pbs_fill alert: fills in 10d, threshold=30d")
	}
}

func TestEvalPBSFill_BeyondThreshold_NoAlert(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	serverID, _ := st.UpsertPBSServer(ctx, "pbs-nofill", "1.1.1.1", "", "1.0", "")
	reportID, _ := st.InsertPBSReport(ctx, serverID)
	fillDate := time.Now().Add(60 * 24 * time.Hour).Unix() // fills in 60 days
	_, _ = st.InsertPBSStore(ctx, reportID, domain.PBSDatastorePayload{
		Store: "ds1", Total: 1000, Used: 200, EstimatedFullDate: fillDate,
	})

	cfg := defaultCfg()
	threshold := 30
	cfg.PBSConfigs[serverID] = domain.PBSAlertConfig{ServerID: serverID, DaysUntilFull: &threshold, VerifyAlert: true}

	alerts, _ := evalPBSFill(st, cfg)
	if hasAlert(alerts, domain.AlertTypePBSFill, "pbs-nofill") {
		t.Error("unexpected pbs_fill alert: fills in 60d, threshold=30d")
	}
}

// ── evalPBSStale ──────────────────────────────────────────────────────────────

func TestEvalPBSStale_OldSnapshot(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	serverID, _ := st.UpsertPBSServer(ctx, "pbs-stale", "1.1.1.1", "", "1.0", "")
	reportID, _ := st.InsertPBSReport(ctx, serverID)
	storeID, _ := st.InsertPBSStore(ctx, reportID, domain.PBSDatastorePayload{Store: "ds1", Total: 1000})
	_ = st.InsertPBSSnapshot(ctx, storeID, domain.PBSGroupPayload{
		BackupType: "vm", BackupID: "101",
		LastBackup: time.Now().Add(-72 * time.Hour).Unix(), // 72h ago
	})

	cfg := defaultCfg() // GlobalStaleHours=48
	alerts, _ := evalPBSStale(st, cfg)
	if !hasAlert(alerts, domain.AlertTypePBSStale, "pbs-stale") {
		t.Error("expected pbs_stale alert for 72h-old snapshot with 48h threshold")
	}
}

func TestEvalPBSStale_RecentSnapshot_NoAlert(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	serverID, _ := st.UpsertPBSServer(ctx, "pbs-fresh", "1.1.1.1", "", "1.0", "")
	reportID, _ := st.InsertPBSReport(ctx, serverID)
	storeID, _ := st.InsertPBSStore(ctx, reportID, domain.PBSDatastorePayload{Store: "ds1", Total: 1000})
	_ = st.InsertPBSSnapshot(ctx, storeID, domain.PBSGroupPayload{
		BackupType: "vm", BackupID: "101",
		LastBackup: time.Now().Add(-10 * time.Hour).Unix(), // 10h ago
	})

	cfg := defaultCfg()
	alerts, _ := evalPBSStale(st, cfg)
	if hasAlert(alerts, domain.AlertTypePBSStale, "pbs-fresh") {
		t.Error("unexpected pbs_stale alert for 10h-old snapshot with 48h threshold")
	}
}

// ── evalPBSVerify ─────────────────────────────────────────────────────────────

func TestEvalPBSVerify_FailedVerification(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	serverID, _ := st.UpsertPBSServer(ctx, "pbs-verify", "1.1.1.1", "", "1.0", "")
	reportID, _ := st.InsertPBSReport(ctx, serverID)
	storeID, _ := st.InsertPBSStore(ctx, reportID, domain.PBSDatastorePayload{Store: "ds1", Total: 1000})
	_ = st.InsertPBSSnapshot(ctx, storeID, domain.PBSGroupPayload{
		BackupType: "vm", BackupID: "101", VerificationState: "failed",
	})

	cfg := defaultCfg()
	cfg.PBSConfigs[serverID] = domain.PBSAlertConfig{ServerID: serverID, VerifyAlert: true}

	alerts, _ := evalPBSVerify(st, cfg)
	if !hasAlert(alerts, domain.AlertTypePBSVerify, "pbs-verify") {
		t.Error("expected pbs_verify alert for failed verification")
	}
}

func TestEvalPBSVerify_OKVerification_NoAlert(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	serverID, _ := st.UpsertPBSServer(ctx, "pbs-okverify", "1.1.1.1", "", "1.0", "")
	reportID, _ := st.InsertPBSReport(ctx, serverID)
	storeID, _ := st.InsertPBSStore(ctx, reportID, domain.PBSDatastorePayload{Store: "ds1", Total: 1000})
	_ = st.InsertPBSSnapshot(ctx, storeID, domain.PBSGroupPayload{
		BackupType: "vm", BackupID: "101", VerificationState: "ok",
	})

	cfg := defaultCfg()
	cfg.PBSConfigs[serverID] = domain.PBSAlertConfig{ServerID: serverID, VerifyAlert: true}

	alerts, _ := evalPBSVerify(st, cfg)
	if hasAlert(alerts, domain.AlertTypePBSVerify, "pbs-okverify") {
		t.Error("unexpected pbs_verify alert for ok verification")
	}
}

func TestEvalPBSVerify_Disabled_NoAlert(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	serverID, _ := st.UpsertPBSServer(ctx, "pbs-noverify", "1.1.1.1", "", "1.0", "")
	reportID, _ := st.InsertPBSReport(ctx, serverID)
	storeID, _ := st.InsertPBSStore(ctx, reportID, domain.PBSDatastorePayload{Store: "ds1", Total: 1000})
	_ = st.InsertPBSSnapshot(ctx, storeID, domain.PBSGroupPayload{
		BackupType: "vm", BackupID: "101", VerificationState: "failed",
	})

	cfg := defaultCfg()
	cfg.PBSConfigs[serverID] = domain.PBSAlertConfig{ServerID: serverID, VerifyAlert: false}

	alerts, _ := evalPBSVerify(st, cfg)
	if hasAlert(alerts, domain.AlertTypePBSVerify, "pbs-noverify") {
		t.Error("unexpected pbs_verify alert when verify_alert=false")
	}
}

// ── RunAll ────────────────────────────────────────────────────────────────────

func TestRunAll_NoServers_NoAlerts(t *testing.T) {
	_, st := openTestStore(t)
	cfg := defaultCfg()
	alerts, err := RunAll(st, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts with no servers, got %d", len(alerts))
	}
}

func TestRunAll_ReturnsAlertsFromMultipleEvaluators(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)

	// PVE server with disk alert + backup error
	pveID, _ := st.UpsertPVEServer(ctx, "pve-all", "1.1.1.1", "", "1.0", "")
	reportID, _ := st.InsertPVEReport(ctx, pveID, nil)
	stgID, _ := st.InsertPVEStorage(ctx, reportID, domain.StoragePayload{Storage: "bkp", Content: "backup"})
	_ = st.InsertPVEStorageInfo(ctx, stgID, domain.StorageInfoPayload{Total: 1000, Used: 900, Avail: 100, Active: true, Enabled: true})
	_ = st.InsertPVEBackupTask(ctx, reportID, domain.BackupTaskPayload{VMID: 101, VMName: "vm1", Status: "ERROR"})

	// PBS server with disk alert
	pbsID, _ := st.UpsertPBSServer(ctx, "pbs-all", "2.2.2.2", "", "1.0", "")
	pbsReportID, _ := st.InsertPBSReport(ctx, pbsID)
	_, _ = st.InsertPBSStore(ctx, pbsReportID, domain.PBSDatastorePayload{Store: "ds1", Total: 1000, Used: 920})

	cfg := defaultCfg()
	alerts, err := RunAll(st, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hasDiskPVE := hasAlert(alerts, domain.AlertTypeDisk, "pve-all")
	hasErrPVE := hasAlert(alerts, domain.AlertTypeBackupError, "pve-all")
	hasDiskPBS := hasAlert(alerts, domain.AlertTypeDisk, "pbs-all")

	if !hasDiskPVE || !hasErrPVE || !hasDiskPBS {
		t.Errorf("missing alerts: disk_pve=%v, err_pve=%v, disk_pbs=%v", hasDiskPVE, hasErrPVE, hasDiskPBS)
	}
}
