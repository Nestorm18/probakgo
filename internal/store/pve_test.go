package store

import (
	"context"
	"testing"
	"time"

	"probakgo/internal/domain"
)

func TestUpsertPVEServer_CreateAndUpdate(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)

	id1, err := st.UpsertPVEServer(ctx, "pve-node", "10.0.0.1", "", "1.0", "mid-aaa")
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	id2, err := st.UpsertPVEServer(ctx, "pve-node", "10.0.0.2", "", "1.1", "mid-aaa")
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	if id1 != id2 {
		t.Errorf("want same ID on upsert, got %d and %d", id1, id2)
	}

	sv, err := st.GetPVEServer(ctx, id1)
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
	ctx := context.Background()
	st := openTestDB(t)
	serverID, _ := st.UpsertPVEServer(ctx, "pve-node", "10.0.0.1", "", "1.0", "")

	reportID, err := st.InsertPVEReport(ctx, serverID, nil)
	if err != nil {
		t.Fatalf("insert report: %v", err)
	}

	rep, err := st.GetLatestPVEReport(ctx, serverID)
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
	ctx := context.Background()
	st := openTestDB(t)
	serverID, _ := st.UpsertPVEServer(ctx, "pve-node", "10.0.0.1", "", "1.0", "")

	id1, _ := st.InsertPVEReport(ctx, serverID, nil)
	// backdate id1 so id2 is definitely the newest by timestamp
	yesterday := time.Now().Add(-24 * time.Hour)
	if _, err := st.db.Exec("UPDATE pve_reports SET reported_at = ? WHERE id = ?", yesterday, id1); err != nil {
		t.Fatalf("backdate report: %v", err)
	}

	id2, _ := st.InsertPVEReport(ctx, serverID, nil)

	rep, err := st.GetLatestPVEReport(ctx, serverID)
	if err != nil {
		t.Fatalf("get latest: %v", err)
	}
	if rep.ID != id2 {
		t.Errorf("want newest report ID %d, got %d", id2, rep.ID)
	}
}

func TestGetLatestPVEReport_NoReports(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	serverID, _ := st.UpsertPVEServer(ctx, "pve-node", "10.0.0.1", "", "1.0", "")

	_, err := st.GetLatestPVEReport(ctx, serverID)
	if err == nil {
		t.Error("want error for server with no reports, got nil")
	}
}

func TestDeleteOldPVEReports(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	serverID, _ := st.UpsertPVEServer(ctx, "pve-node", "10.0.0.1", "", "1.0", "")

	// Insert old report and backdate it to 6 months ago
	oldID, _ := st.InsertPVEReport(ctx, serverID, nil)
	sixMonthsAgo := time.Now().AddDate(0, -6, 0)
	if _, err := st.db.Exec("UPDATE pve_reports SET reported_at = ? WHERE id = ?", sixMonthsAgo, oldID); err != nil {
		t.Fatalf("backdate old report: %v", err)
	}

	// Add storage + content children to verify cascade delete
	stID, _ := st.InsertPVEStorage(ctx, oldID, domain.StoragePayload{Storage: "local", Content: "backup", Type: "dir"})
	_ = st.InsertPVEStorageContent(ctx, stID, domain.ContentDataPayload{VMID: 100, Format: "tar"})

	// Insert a current report
	_, _ = st.InsertPVEReport(ctx, serverID, nil)

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
	if _, err := st.GetLatestPVEReport(ctx, serverID); err != nil {
		t.Fatalf("current report should remain: %v", err)
	}
}

func TestInsertPVEBackupTask_RoundTrip(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	serverID, _ := st.UpsertPVEServer(ctx, "pve-node", "10.0.0.1", "", "1.0", "")
	reportID, _ := st.InsertPVEReport(ctx, serverID, nil)

	task := domain.BackupTaskPayload{
		VMID:      100,
		VMName:    "debian-vm",
		Status:    "OK",
		StartTime: 1000,
		EndTime:   2000,
		Duration:  1000,
		Size:      512 * 1024 * 1024,
		Filename:  "vzdump-qemu-100.vma.zst",
	}
	if err := st.InsertPVEBackupTask(ctx, reportID, task); err != nil {
		t.Fatalf("insert task: %v", err)
	}

	tasks, err := st.GetPVEBackupTasksForReport(ctx, reportID)
	if err != nil {
		t.Fatalf("get tasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("want 1 task, got %d", len(tasks))
	}
	got := tasks[0]
	if got.ReportID != reportID {
		t.Errorf("ReportID: want %d, got %d", reportID, got.ReportID)
	}
	if got.VMID != 100 {
		t.Errorf("VMID: want 100, got %d", got.VMID)
	}
	if got.VMName != "debian-vm" {
		t.Errorf("VMName: want debian-vm, got %q", got.VMName)
	}
	if got.Status != "OK" {
		t.Errorf("Status: want OK, got %q", got.Status)
	}
	if got.Size != 512*1024*1024 {
		t.Errorf("Size: want %d, got %d", 512*1024*1024, got.Size)
	}
	if got.Filename != "vzdump-qemu-100.vma.zst" {
		t.Errorf("Filename: want vzdump-qemu-100.vma.zst, got %q", got.Filename)
	}
}

func TestGetPVEBackupTasksForReport_Empty(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	serverID, _ := st.UpsertPVEServer(ctx, "pve-node", "10.0.0.1", "", "1.0", "")
	reportID, _ := st.InsertPVEReport(ctx, serverID, nil)

	tasks, err := st.GetPVEBackupTasksForReport(ctx, reportID)
	if err != nil {
		t.Fatalf("get tasks: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("want 0 tasks for report with no tasks, got %d", len(tasks))
	}
}

func TestGetPVEBackupTasksForReport_OrderedByStarttime(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	serverID, _ := st.UpsertPVEServer(ctx, "pve-node", "10.0.0.1", "", "1.0", "")
	reportID, _ := st.InsertPVEReport(ctx, serverID, nil)

	_ = st.InsertPVEBackupTask(ctx, reportID, domain.BackupTaskPayload{VMID: 200, StartTime: 2000})
	_ = st.InsertPVEBackupTask(ctx, reportID, domain.BackupTaskPayload{VMID: 100, StartTime: 1000})

	tasks, err := st.GetPVEBackupTasksForReport(ctx, reportID)
	if err != nil {
		t.Fatalf("get tasks: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("want 2 tasks, got %d", len(tasks))
	}
	if tasks[0].VMID != 100 || tasks[1].VMID != 200 {
		t.Errorf("want tasks ordered by starttime ASC, got VMIDs %d, %d", tasks[0].VMID, tasks[1].VMID)
	}
}
