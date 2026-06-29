package store

import (
	"context"
	"testing"
	"time"

	"probakgo/internal/domain"
)

func TestUpsertPBSServer_CreateAndUpdate(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)

	id1, err := st.UpsertPBSServer(ctx, "pbs-node", "10.0.1.1", "", "1.0", "mid-bbb")
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	id2, err := st.UpsertPBSServer(ctx, "pbs-node", "10.0.1.2", "", "1.1", "mid-bbb")
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	if id1 != id2 {
		t.Errorf("want same ID on upsert, got %d and %d", id1, id2)
	}

	sv, err := st.GetPBSServer(ctx, id1)
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
	ctx := context.Background()
	st := openTestDB(t)
	serverID, _ := st.UpsertPBSServer(ctx, "pbs-node", "10.0.1.1", "", "1.0", "")

	id1, _ := st.InsertPBSReport(ctx, serverID)
	// backdate id1 so id2 is definitely the newest by timestamp
	yesterday := time.Now().Add(-24 * time.Hour)
	if _, err := st.db.Exec("UPDATE pbs_reports SET reported_at = ? WHERE id = ?", yesterday, id1); err != nil {
		t.Fatalf("backdate report: %v", err)
	}

	id2, _ := st.InsertPBSReport(ctx, serverID)

	rep, err := st.GetLatestPBSReport(ctx, serverID)
	if err != nil {
		t.Fatalf("get latest: %v", err)
	}
	if rep.ID != id2 {
		t.Errorf("want newest report ID %d, got %d", id2, rep.ID)
	}
}

func TestDeleteOldPBSReports(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	serverID, _ := st.UpsertPBSServer(ctx, "pbs-node", "10.0.1.1", "", "1.0", "")

	// Insert old report and backdate it
	oldID, _ := st.InsertPBSReport(ctx, serverID)
	sixMonthsAgo := time.Now().AddDate(0, -6, 0)
	if _, err := st.db.Exec("UPDATE pbs_reports SET reported_at = ? WHERE id = ?", sixMonthsAgo, oldID); err != nil {
		t.Fatalf("backdate old report: %v", err)
	}

	// Add store + gc_status children to verify cascade delete
	stID, _ := st.InsertPBSStore(ctx, oldID, domain.PBSDatastorePayload{Store: "backup", Total: 1000, Used: 500})
	_ = st.InsertPBSGCStatus(ctx, stID, &domain.GCStatusPayload{DiskBytes: 100, DiskChunks: 5})

	// Insert a current report
	_, _ = st.InsertPBSReport(ctx, serverID)

	cutoff := time.Now().AddDate(0, -1, 0)
	n, err := st.DeleteOldPBSReports(ctx, cutoff)
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
	if _, err := st.GetLatestPBSReport(ctx, serverID); err != nil {
		t.Fatalf("current report should remain: %v", err)
	}
}

func TestInsertPBSSnapshot_And_GetSnapshotsForStore(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	serverID, _ := st.UpsertPBSServer(ctx, "pbs-node", "10.0.1.1", "", "1.0", "")
	reportID, _ := st.InsertPBSReport(ctx, serverID)
	storeID, _ := st.InsertPBSStore(ctx, reportID, domain.PBSDatastorePayload{Store: "datastore1", Total: 1000})

	g1 := domain.PBSGroupPayload{BackupType: "ct", BackupID: "101", BackupCount: 5, LastBackup: 1000}
	g2 := domain.PBSGroupPayload{BackupType: "vm", BackupID: "100", BackupCount: 3, LastBackup: 2000, VerificationState: "ok"}

	if err := st.InsertPBSSnapshot(ctx, storeID, g1); err != nil {
		t.Fatalf("InsertPBSSnapshot g1: %v", err)
	}
	if err := st.InsertPBSSnapshot(ctx, storeID, g2); err != nil {
		t.Fatalf("InsertPBSSnapshot g2: %v", err)
	}

	snaps, err := st.GetPBSSnapshotsForStore(ctx, storeID)
	if err != nil {
		t.Fatalf("GetPBSSnapshotsForStore: %v", err)
	}
	if len(snaps) != 2 {
		t.Fatalf("want 2 snapshots, got %d", len(snaps))
	}
	// Ordered by backup_type, backup_id: ct < vm
	if snaps[0].BackupType != "ct" || snaps[0].BackupID != "101" {
		t.Errorf("want first snap ct/101, got %s/%s", snaps[0].BackupType, snaps[0].BackupID)
	}
	if snaps[1].BackupType != "vm" || snaps[1].VerificationState != "ok" {
		t.Errorf("want second snap vm with verification ok, got %s ver=%s", snaps[1].BackupType, snaps[1].VerificationState)
	}
}

func TestGetPBSHistory(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	serverID, _ := st.UpsertPBSServer(ctx, "pbs-node", "10.0.1.1", "", "1.0", "")
	reportID, _ := st.InsertPBSReport(ctx, serverID)
	storeID, _ := st.InsertPBSStore(ctx, reportID, domain.PBSDatastorePayload{Store: "ds1", Total: 1000})

	v := 42.5
	history := []*float64{&v, nil, &v}
	if err := st.InsertPBSStoreHistory(ctx, storeID, history); err != nil {
		t.Fatalf("InsertPBSStoreHistory: %v", err)
	}

	got, err := st.GetPBSHistory(ctx, storeID)
	if err != nil {
		t.Fatalf("GetPBSHistory: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 history entries, got %d", len(got))
	}
	if got[0] == nil || *got[0] != 42.5 {
		t.Errorf("history[0]: want 42.5, got %v", got[0])
	}
	if got[1] != nil {
		t.Errorf("history[1]: want nil, got %v", got[1])
	}
	if got[2] == nil || *got[2] != 42.5 {
		t.Errorf("history[2]: want 42.5, got %v", got[2])
	}
}

func TestListPBSReports_LimitAndOrder(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	serverID, _ := st.UpsertPBSServer(ctx, "pbs-node", "10.0.1.1", "", "1.0", "")

	id1, _ := st.InsertPBSReport(ctx, serverID)
	id2, _ := st.InsertPBSReport(ctx, serverID)
	id3, _ := st.InsertPBSReport(ctx, serverID)
	st.db.Exec("UPDATE pbs_reports SET reported_at = ? WHERE id = ?", time.Now().Add(-72*time.Hour), id1)
	st.db.Exec("UPDATE pbs_reports SET reported_at = ? WHERE id = ?", time.Now().Add(-48*time.Hour), id2)

	reports, err := st.ListPBSReports(ctx, serverID, 2)
	if err != nil {
		t.Fatalf("ListPBSReports: %v", err)
	}
	if len(reports) != 2 {
		t.Fatalf("want 2 reports (limit), got %d", len(reports))
	}
	if reports[0].ID != id3 || reports[1].ID != id2 {
		t.Errorf("want newest first: [%d,%d], got [%d,%d]", id3, id2, reports[0].ID, reports[1].ID)
	}
}

func TestListPBSReportsPageAndCount(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	serverID, _ := st.UpsertPBSServer(ctx, "pbs-node", "10.0.1.1", "", "1.0", "")

	id1, _ := st.InsertPBSReport(ctx, serverID)
	id2, _ := st.InsertPBSReport(ctx, serverID)
	id3, _ := st.InsertPBSReport(ctx, serverID)
	st.db.Exec("UPDATE pbs_reports SET reported_at = ? WHERE id = ?", time.Now().Add(-72*time.Hour), id1)
	st.db.Exec("UPDATE pbs_reports SET reported_at = ? WHERE id = ?", time.Now().Add(-48*time.Hour), id2)

	count, err := st.CountPBSReports(ctx, serverID)
	if err != nil {
		t.Fatalf("CountPBSReports: %v", err)
	}
	if count != 3 {
		t.Fatalf("want 3 reports, got %d", count)
	}

	reports, err := st.ListPBSReportsPage(ctx, serverID, 2, 1)
	if err != nil {
		t.Fatalf("ListPBSReportsPage: %v", err)
	}
	if len(reports) != 2 {
		t.Fatalf("want 2 reports, got %d", len(reports))
	}
	if reports[0].ID != id2 || reports[1].ID != id1 || reports[0].ID == id3 {
		t.Fatalf("unexpected page order: got [%d,%d], newest id=%d", reports[0].ID, reports[1].ID, id3)
	}
}

func TestDeletePBSServer_SoftDelete(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	id, _ := st.UpsertPBSServer(ctx, "pbs-node", "10.0.1.1", "", "1.0", "")

	if err := st.DeletePBSServer(ctx, id); err != nil {
		t.Fatalf("DeletePBSServer: %v", err)
	}

	if _, err := st.GetPBSServer(ctx, id); err == nil {
		t.Error("want error for deleted server from GetPBSServer")
	}

	servers, err := st.ListPBSServers(ctx)
	if err != nil {
		t.Fatalf("ListPBSServers: %v", err)
	}
	if len(servers) != 0 {
		t.Errorf("want 0 servers after delete, got %d", len(servers))
	}

	var count int
	st.db.QueryRow("SELECT COUNT(*) FROM pbs_servers WHERE id=? AND is_deleted=1", id).Scan(&count)
	if count != 1 {
		t.Error("want soft-deleted row to remain in DB with is_deleted=1")
	}
}
