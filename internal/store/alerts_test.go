package store

import (
	"encoding/json"
	"testing"

	"probakgo/internal/domain"
)

// seedPVEWithBackupStorage inserts a PVE server with one backup storage at the given usage %.
func seedPVEWithBackupStorage(t *testing.T, st *Store, serverName string, usedPct int) {
	t.Helper()
	serverID, _ := st.UpsertPVEServer(serverName, "10.0.0.1", "", "1.0", "")
	reportID, _ := st.InsertPVEReport(serverID, nil)
	stID, _ := st.InsertPVEStorage(reportID, domain.StoragePayload{
		Storage: "local-bak",
		Content: "backup",
		Type:    "dir",
	})
	total := int64(1000)
	used := total * int64(usedPct) / 100
	_ = st.InsertPVEStorageInfo(stID, domain.StorageInfoPayload{
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
	serverID, _ := st.UpsertPBSServer(serverName, "10.0.0.1", "", "1.0", "")
	reportID, _ := st.InsertPBSReport(serverID)
	total := int64(1000)
	used := total * int64(usedPct) / 100
	_, _ = st.InsertPBSStore(reportID, domain.PBSDatastorePayload{
		Store: "backup",
		Total: total,
		Used:  used,
		Avail: total - used,
	})
}

func TestGetAlerts_DiskPctZero_NoCheck(t *testing.T) {
	st := openTestDB(t)
	seedPBSWithStore(t, st, "pbs-full", 95)
	seedPVEWithBackupStorage(t, st, "pve-full", 95)

	alerts, err := st.GetAlerts(0, false)
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
	st := openTestDB(t)
	seedPBSWithStore(t, st, "pbs-heavy", 90)

	alerts, err := st.GetAlerts(85, false)
	if err != nil {
		t.Fatalf("get alerts: %v", err)
	}

	found := false
	for _, a := range alerts {
		if a.Type == "disk" && a.ServerName == "pbs-heavy" {
			found = true
			if a.UsedPct < 85 {
				t.Errorf("UsedPct: want >= 85, got %d", a.UsedPct)
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
	st := openTestDB(t)
	seedPBSWithStore(t, st, "pbs-light", 50)

	alerts, err := st.GetAlerts(85, false)
	if err != nil {
		t.Fatalf("get alerts: %v", err)
	}
	for _, a := range alerts {
		if a.Type == "disk" && a.ServerName == "pbs-light" {
			t.Error("expected no disk alert for pbs-light at 50% usage")
		}
	}
}

func TestGetAlerts_PVEDisk_OverThreshold(t *testing.T) {
	st := openTestDB(t)
	seedPVEWithBackupStorage(t, st, "pve-heavy", 90)

	alerts, err := st.GetAlerts(85, false)
	if err != nil {
		t.Fatalf("get alerts: %v", err)
	}

	found := false
	for _, a := range alerts {
		if a.Type == "disk" && a.ServerName == "pve-heavy" {
			found = true
		}
	}
	if !found {
		t.Error("expected disk alert for pve-heavy, got none")
	}
}

func TestGetAlerts_BackupError(t *testing.T) {
	st := openTestDB(t)
	serverID, _ := st.UpsertPVEServer("pve-err", "10.0.0.1", "", "1.0", "")
	bs := &domain.BackupStatus{Status: json.RawMessage(`"ERROR"`)}
	_, _ = st.InsertPVEReport(serverID, bs)

	alerts, err := st.GetAlerts(0, true)
	if err != nil {
		t.Fatalf("get alerts: %v", err)
	}

	found := false
	for _, a := range alerts {
		if a.Type == "backup_error" && a.ServerName == "pve-err" {
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
	st := openTestDB(t)
	serverID, _ := st.UpsertPVEServer("pve-ok", "10.0.0.1", "", "1.0", "")
	bs := &domain.BackupStatus{Status: json.RawMessage(`"OK"`)}
	_, _ = st.InsertPVEReport(serverID, bs)

	alerts, err := st.GetAlerts(0, true)
	if err != nil {
		t.Fatalf("get alerts: %v", err)
	}
	for _, a := range alerts {
		if a.Type == "backup_error" && a.ServerName == "pve-ok" {
			t.Error("expected no backup_error alert for OK status")
		}
	}
}
