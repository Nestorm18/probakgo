package store

import (
	"context"
	"testing"
	"time"

	"probakgo/internal/domain"
)

func TestDeleteOldWindowsReports(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)

	serverID, err := st.UpsertWindowsServer(ctx, "win1", "1.1.1.1", "", "1.0", "machine-win")
	if err != nil {
		t.Fatalf("UpsertWindowsServer: %v", err)
	}
	oldReportID, err := st.InsertWindowsReport(ctx, serverID)
	if err != nil {
		t.Fatalf("InsertWindowsReport old: %v", err)
	}
	newReportID, err := st.InsertWindowsReport(ctx, serverID)
	if err != nil {
		t.Fatalf("InsertWindowsReport new: %v", err)
	}
	if err := st.InsertWindowsDisk(ctx, oldReportID, domain.WindowsDiskPayload{Name: "C:", Total: 100, Used: 90}); err != nil {
		t.Fatalf("InsertWindowsDisk old: %v", err)
	}
	if err := st.InsertWindowsDisk(ctx, newReportID, domain.WindowsDiskPayload{Name: "C:", Total: 100, Used: 50}); err != nil {
		t.Fatalf("InsertWindowsDisk new: %v", err)
	}
	oldTime := time.Now().AddDate(0, -2, 0)
	if _, err := st.db.Exec(`UPDATE windows_reports SET reported_at = ? WHERE id = ?`, oldTime, oldReportID); err != nil {
		t.Fatalf("set old report time: %v", err)
	}

	n, err := st.DeleteOldWindowsReports(ctx, time.Now().AddDate(0, -1, 0))
	if err != nil {
		t.Fatalf("DeleteOldWindowsReports: %v", err)
	}
	if n != 1 {
		t.Fatalf("deleted reports: got %d, want 1", n)
	}
	var count int
	if err := st.db.QueryRow(`SELECT COUNT(*) FROM windows_reports`).Scan(&count); err != nil {
		t.Fatalf("count windows_reports: %v", err)
	}
	if count != 1 {
		t.Fatalf("windows_reports count: got %d, want 1", count)
	}
	if err := st.db.QueryRow(`SELECT COUNT(*) FROM windows_disks`).Scan(&count); err != nil {
		t.Fatalf("count windows_disks: %v", err)
	}
	if count != 1 {
		t.Fatalf("windows_disks count: got %d, want 1", count)
	}
}

func TestListWindowsReportsPageAndCount(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)

	serverID, err := st.UpsertWindowsServer(ctx, "win-page", "1.1.1.1", "", "1.0", "machine-win")
	if err != nil {
		t.Fatalf("UpsertWindowsServer: %v", err)
	}
	for i := 0; i < 3; i++ {
		if _, err := st.InsertWindowsReport(ctx, serverID); err != nil {
			t.Fatalf("InsertWindowsReport %d: %v", i, err)
		}
		time.Sleep(time.Millisecond)
	}

	total, err := st.CountWindowsReports(ctx, serverID)
	if err != nil {
		t.Fatalf("CountWindowsReports: %v", err)
	}
	if total != 3 {
		t.Fatalf("count: got %d, want 3", total)
	}
	reports, err := st.ListWindowsReportsPage(ctx, serverID, 2, 1)
	if err != nil {
		t.Fatalf("ListWindowsReportsPage: %v", err)
	}
	if len(reports) != 2 {
		t.Fatalf("reports: got %d, want 2", len(reports))
	}
}

func TestListWindowsReportsByDays(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)

	serverID, err := st.UpsertWindowsServer(ctx, "win-days", "1.1.1.1", "", "1.0", "machine-win")
	if err != nil {
		t.Fatalf("UpsertWindowsServer: %v", err)
	}
	oldID, err := st.InsertWindowsReport(ctx, serverID)
	if err != nil {
		t.Fatalf("InsertWindowsReport old: %v", err)
	}
	newID, err := st.InsertWindowsReport(ctx, serverID)
	if err != nil {
		t.Fatalf("InsertWindowsReport new: %v", err)
	}
	if _, err := st.db.Exec(`UPDATE windows_reports SET reported_at = ? WHERE id = ?`, time.Now().AddDate(0, 0, -40), oldID); err != nil {
		t.Fatalf("set old report time: %v", err)
	}

	reports, err := st.ListWindowsReportsByDays(ctx, serverID, 30)
	if err != nil {
		t.Fatalf("ListWindowsReportsByDays: %v", err)
	}
	if len(reports) != 1 || reports[0].ID != newID {
		t.Fatalf("reports: got %+v, want only report %d", reports, newID)
	}
}
