package store

import (
	"testing"
	"time"

	"probakgo/internal/domain"
)

func TestUpsertPVEServer_CreateAndUpdate(t *testing.T) {
	st := openTestDB(t)

	id1, err := st.UpsertPVEServer("pve-node", "10.0.0.1", "", "1.0", "mid-aaa")
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	id2, err := st.UpsertPVEServer("pve-node", "10.0.0.2", "", "1.1", "mid-aaa")
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	if id1 != id2 {
		t.Errorf("want same ID on upsert, got %d and %d", id1, id2)
	}

	sv, err := st.GetPVEServer(id1)
	if err != nil {
		t.Fatalf("get server: %v", err)
	}
	if sv.IP != "10.0.0.2" {
		t.Errorf("IP: want 10.0.0.2, got %s", sv.IP)
	}
	if sv.ClientVersion != "1.1" {
		t.Errorf("ClientVersion: want 1.1, got %s", sv.ClientVersion)
	}
}

func TestInsertPVEReport_NilBackupStatus(t *testing.T) {
	st := openTestDB(t)
	serverID, _ := st.UpsertPVEServer("pve-node", "10.0.0.1", "", "1.0", "")

	reportID, err := st.InsertPVEReport(serverID, nil)
	if err != nil {
		t.Fatalf("insert report: %v", err)
	}

	rep, err := st.GetLatestPVEReport(serverID)
	if err != nil {
		t.Fatalf("get latest: %v", err)
	}
	if rep.ID != reportID {
		t.Errorf("ID: want %d, got %d", reportID, rep.ID)
	}
	if rep.BackupStatus != "" {
		t.Errorf("BackupStatus: want empty, got %q", rep.BackupStatus)
	}
}

func TestGetLatestPVEReport_ReturnsNewest(t *testing.T) {
	st := openTestDB(t)
	serverID, _ := st.UpsertPVEServer("pve-node", "10.0.0.1", "", "1.0", "")

	id1, _ := st.InsertPVEReport(serverID, nil)
	// backdate id1 so id2 is definitely the newest by timestamp
	yesterday := time.Now().Add(-24 * time.Hour)
	if _, err := st.db.Exec("UPDATE pve_reports SET reported_at = ? WHERE id = ?", yesterday, id1); err != nil {
		t.Fatalf("backdate report: %v", err)
	}

	id2, _ := st.InsertPVEReport(serverID, nil)

	rep, err := st.GetLatestPVEReport(serverID)
	if err != nil {
		t.Fatalf("get latest: %v", err)
	}
	if rep.ID != id2 {
		t.Errorf("want newest report ID %d, got %d", id2, rep.ID)
	}
}

func TestGetLatestPVEReport_NoReports(t *testing.T) {
	st := openTestDB(t)
	serverID, _ := st.UpsertPVEServer("pve-node", "10.0.0.1", "", "1.0", "")

	_, err := st.GetLatestPVEReport(serverID)
	if err == nil {
		t.Error("want error for server with no reports, got nil")
	}
}

func TestDeleteOldPVEReports(t *testing.T) {
	st := openTestDB(t)
	serverID, _ := st.UpsertPVEServer("pve-node", "10.0.0.1", "", "1.0", "")

	// Insert old report and backdate it to 6 months ago
	oldID, _ := st.InsertPVEReport(serverID, nil)
	sixMonthsAgo := time.Now().AddDate(0, -6, 0)
	if _, err := st.db.Exec("UPDATE pve_reports SET reported_at = ? WHERE id = ?", sixMonthsAgo, oldID); err != nil {
		t.Fatalf("backdate old report: %v", err)
	}

	// Add storage + content children to verify cascade delete
	stID, _ := st.InsertPVEStorage(oldID, domain.StoragePayload{Storage: "local", Content: "backup", Type: "dir"})
	_ = st.InsertPVEStorageContent(stID, domain.ContentDataPayload{VMID: 100, Format: "tar"})

	// Insert a current report
	_, _ = st.InsertPVEReport(serverID, nil)

	cutoff := time.Now().AddDate(0, -1, 0)
	n, err := st.DeleteOldPVEReports(cutoff)
	if err != nil {
		t.Fatalf("delete old reports: %v", err)
	}
	if n != 1 {
		t.Errorf("want 1 deleted, got %d", n)
	}

	var count int
	st.db.QueryRow("SELECT COUNT(*) FROM pve_reports WHERE id = ?", oldID).Scan(&count)
	if count != 0 {
		t.Error("old report should be deleted")
	}
	st.db.QueryRow("SELECT COUNT(*) FROM pve_storage_content WHERE storage_id = ?", stID).Scan(&count)
	if count != 0 {
		t.Error("content should be cascade-deleted with the old report")
	}

	// Current report must survive
	if _, err := st.GetLatestPVEReport(serverID); err != nil {
		t.Fatalf("current report should remain: %v", err)
	}
}
