package store

import (
	"testing"
	"time"

	"probakgo/internal/domain"
)

func TestUpsertPBSServer_CreateAndUpdate(t *testing.T) {
	st := openTestDB(t)

	id1, err := st.UpsertPBSServer("pbs-node", "10.0.1.1", "", "1.0", "mid-bbb")
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	id2, err := st.UpsertPBSServer("pbs-node", "10.0.1.2", "", "1.1", "mid-bbb")
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	if id1 != id2 {
		t.Errorf("want same ID on upsert, got %d and %d", id1, id2)
	}

	sv, err := st.GetPBSServer(id1)
	if err != nil {
		t.Fatalf("get server: %v", err)
	}
	if sv.IP != "10.0.1.2" {
		t.Errorf("IP: want 10.0.1.2, got %s", sv.IP)
	}
	if sv.ClientVersion != "1.1" {
		t.Errorf("ClientVersion: want 1.1, got %s", sv.ClientVersion)
	}
}

func TestInsertPBSReport_And_GetLatest(t *testing.T) {
	st := openTestDB(t)
	serverID, _ := st.UpsertPBSServer("pbs-node", "10.0.1.1", "", "1.0", "")

	id1, _ := st.InsertPBSReport(serverID)
	// backdate id1 so id2 is definitely the newest by timestamp
	yesterday := time.Now().Add(-24 * time.Hour)
	if _, err := st.db.Exec("UPDATE pbs_reports SET reported_at = ? WHERE id = ?", yesterday, id1); err != nil {
		t.Fatalf("backdate report: %v", err)
	}

	id2, _ := st.InsertPBSReport(serverID)

	rep, err := st.GetLatestPBSReport(serverID)
	if err != nil {
		t.Fatalf("get latest: %v", err)
	}
	if rep.ID != id2 {
		t.Errorf("want newest report ID %d, got %d", id2, rep.ID)
	}
}

func TestDeleteOldPBSReports(t *testing.T) {
	st := openTestDB(t)
	serverID, _ := st.UpsertPBSServer("pbs-node", "10.0.1.1", "", "1.0", "")

	// Insert old report and backdate it
	oldID, _ := st.InsertPBSReport(serverID)
	sixMonthsAgo := time.Now().AddDate(0, -6, 0)
	if _, err := st.db.Exec("UPDATE pbs_reports SET reported_at = ? WHERE id = ?", sixMonthsAgo, oldID); err != nil {
		t.Fatalf("backdate old report: %v", err)
	}

	// Add store + gc_status children to verify cascade delete
	stID, _ := st.InsertPBSStore(oldID, domain.PBSDatastorePayload{Store: "backup", Total: 1000, Used: 500})
	_ = st.InsertPBSGCStatus(stID, &domain.GCStatusPayload{DiskBytes: 100, DiskChunks: 5})

	// Insert a current report
	_, _ = st.InsertPBSReport(serverID)

	cutoff := time.Now().AddDate(0, -1, 0)
	n, err := st.DeleteOldPBSReports(cutoff)
	if err != nil {
		t.Fatalf("delete old reports: %v", err)
	}
	if n != 1 {
		t.Errorf("want 1 deleted, got %d", n)
	}

	var count int
	st.db.QueryRow("SELECT COUNT(*) FROM pbs_reports WHERE id = ?", oldID).Scan(&count)
	if count != 0 {
		t.Error("old report should be deleted")
	}
	st.db.QueryRow("SELECT COUNT(*) FROM pbs_gc_status WHERE store_id = ?", stID).Scan(&count)
	if count != 0 {
		t.Error("gc_status should be cascade-deleted with the old report")
	}

	// Current report must survive
	if _, err := st.GetLatestPBSReport(serverID); err != nil {
		t.Fatalf("current report should remain: %v", err)
	}
}
