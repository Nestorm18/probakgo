package store

import (
	"context"
	"encoding/json"
	"testing"

	"probakgo/internal/domain"
)

// seedPVEWithBackupStorage inserts a PVE server with one backup storage at the given usage %.
func seedPVEWithBackupStorage(t *testing.T, st *Store, serverName string, usedPct int) {
	t.Helper()
	ctx := context.Background()
	serverID, _ := st.UpsertPVEServer(ctx, serverName, "10.0.0.1", "", "1.0", "")
	reportID, _ := st.InsertPVEReport(ctx, serverID, nil)
	stID, _ := st.InsertPVEStorage(ctx, reportID, domain.StoragePayload{
		Storage: "local-bak",
		Content: "backup",
		Type:    "dir",
	})
	total := int64(1000)
	used := total * int64(usedPct) / 100
	_ = st.InsertPVEStorageInfo(ctx, stID, domain.StorageInfoPayload{
		Total:   total,
		Used:    used,
		Avail:   total - used,
		UsedPct: float64(usedPct),
		Active:  true,
		Enabled: true,
	})
}

// seedPBSWithStore inserts a PBS server with one datastore at the given usage %.
func seedPBSWithStore(t *testing.T, st *Store, serverName string, usedPct int) {
	t.Helper()
	ctx := context.Background()
	serverID, _ := st.UpsertPBSServer(ctx, serverName, "10.0.0.1", "", "1.0", "")
	reportID, _ := st.InsertPBSReport(ctx, serverID)
	total := int64(1000)
	used := total * int64(usedPct) / 100
	_, _ = st.InsertPBSStore(ctx, reportID, domain.PBSDatastorePayload{
		Store: "backup",
		Total: total,
		Used:  used,
		Avail: total - used,
	})
}

func TestGetAlerts_DiskPctZero_NoCheck(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	seedPBSWithStore(t, st, "pbs-full", 95)
	seedPVEWithBackupStorage(t, st, "pve-full", 95)

	alerts, err := st.GetAlerts(ctx, 0, false)
	if err != nil {
		t.Fatalf("get alerts: %v", err)
	}
	for _, a := range alerts {
		if a.Type == "disk" {
			t.Errorf("expected no disk alerts when diskPct=0, got one for %s", a.ServerName)
		}
	}
}

func TestGetAlerts_PBSDisk_OverThreshold(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	seedPBSWithStore(t, st, "pbs-heavy", 90)

	alerts, err := st.GetAlerts(ctx, 85, false)
	if err != nil {
		t.Fatalf("get alerts: %v", err)
	}

	found := false
	for _, a := range alerts {
		if a.Type == domain.AlertTypeDisk && a.ServerName == "pbs-heavy" {
			found = true
			if a.Value == "" {
				t.Error("Value should not be empty for disk alert")
			}
			if a.Message == "" {
				t.Error("Message should not be empty")
			}
		}
	}
	if !found {
		t.Error("expected disk alert for pbs-heavy, got none")
	}
}

func TestGetAlerts_PBSDisk_UnderThreshold(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	seedPBSWithStore(t, st, "pbs-light", 50)

	alerts, err := st.GetAlerts(ctx, 85, false)
	if err != nil {
		t.Fatalf("get alerts: %v", err)
	}
	for _, a := range alerts {
		if a.Type == domain.AlertTypeDisk && a.ServerName == "pbs-light" {
			t.Error("expected no disk alert for pbs-light at 50% usage")
		}
	}
}

func TestGetAlerts_PVEDisk_OverThreshold(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	seedPVEWithBackupStorage(t, st, "pve-heavy", 90)

	alerts, err := st.GetAlerts(ctx, 85, false)
	if err != nil {
		t.Fatalf("get alerts: %v", err)
	}

	found := false
	for _, a := range alerts {
		if a.Type == domain.AlertTypeDisk && a.ServerName == "pve-heavy" {
			found = true
		}
	}
	if !found {
		t.Error("expected disk alert for pve-heavy, got none")
	}
}

func TestGetAlerts_BackupError(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	serverID, _ := st.UpsertPVEServer(ctx, "pve-err", "10.0.0.1", "", "1.0", "")
	bs := &domain.BackupStatus{Status: json.RawMessage(`"ERROR"`)}
	_, _ = st.InsertPVEReport(ctx, serverID, bs)

	alerts, err := st.GetAlerts(ctx, 0, true)
	if err != nil {
		t.Fatalf("get alerts: %v", err)
	}

	found := false
	for _, a := range alerts {
		if a.Type == domain.AlertTypeBackupError && a.ServerName == "pve-err" {
			found = true
			if a.Message == "" {
				t.Error("backup_error alert Message should not be empty")
			}
		}
	}
	if !found {
		t.Error("expected backup_error alert for pve-err, got none")
	}
}

func TestGetAlerts_BackupOK_NoAlert(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	serverID, _ := st.UpsertPVEServer(ctx, "pve-ok", "10.0.0.1", "", "1.0", "")
	bs := &domain.BackupStatus{Status: json.RawMessage(`"OK"`)}
	_, _ = st.InsertPVEReport(ctx, serverID, bs)

	alerts, err := st.GetAlerts(ctx, 0, true)
	if err != nil {
		t.Fatalf("get alerts: %v", err)
	}
	for _, a := range alerts {
		if a.Type == domain.AlertTypeBackupError && a.ServerName == "pve-ok" {
			t.Error("expected no backup_error alert for OK status")
		}
	}
}
