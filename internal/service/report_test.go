package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"probakgo/internal/domain"
)

func TestIsStale_TodayNotStale(t *testing.T) {
	_, st := openTestStore(t)
	svc := NewReport(st, time.UTC)

	if svc.IsStale(time.Now()) {
		t.Error("report from now should not be stale")
	}
}

func TestIsStale_YesterdayStale(t *testing.T) {
	_, st := openTestStore(t)
	svc := NewReport(st, time.UTC)

	yesterday := time.Now().Add(-25 * time.Hour)
	if !svc.IsStale(yesterday) {
		t.Error("report from yesterday should be stale")
	}
}

func TestSavePVEReport_FullRoundTrip(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	svc := NewReport(st, time.UTC)

	statusJSON, _ := json.Marshal("OK")
	req := &domain.PVEReportRequest{
		Hostname:      "pve-01",
		IPAddress:     "10.0.0.1",
		ClientVersion: "1.0",
		LastBackupStatus: &domain.BackupStatus{
			Status:    statusJSON,
			StartTime: 1000,
			EndTime:   2000,
			Duration:  1000,
		},
		Storages: []domain.StoragePayload{
			{
				Storage: "local",
				Content: "backup",
				Type:    "dir",
				StorageInfo: []domain.StorageInfoPayload{
					{Total: 100, Used: 50, Avail: 50, UsedPct: 50.0, Active: true, Enabled: true},
				},
				ContentData: []domain.ContentDataPayload{
					{VMID: 101, Format: "tar", Verification: "ok"},
				},
			},
		},
	}

	if err := svc.SavePVEReport(ctx, req); err != nil {
		t.Fatalf("SavePVEReport: %v", err)
	}

	servers, err := st.ListPVEServers(ctx)
	if err != nil {
		t.Fatalf("ListPVEServers: %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("want 1 server, got %d", len(servers))
	}

	rep, err := st.GetLatestPVEReport(ctx, servers[0].ID)
	if err != nil {
		t.Fatalf("GetLatestPVEReport: %v", err)
	}
	if rep.BackupStatus != "OK" {
		t.Errorf("BackupStatus: want OK, got %q", rep.BackupStatus)
	}
	if rep.BackupDuration != 1000 {
		t.Errorf("BackupDuration: want 1000, got %d", rep.BackupDuration)
	}

	storages, err := st.GetPVEStoragesForReport(ctx, rep.ID)
	if err != nil {
		t.Fatalf("GetPVEStoragesForReport: %v", err)
	}
	if len(storages) != 1 {
		t.Fatalf("want 1 storage, got %d", len(storages))
	}

	content, err := st.GetPVEStorageContent(ctx, storages[0].ID)
	if err != nil {
		t.Fatalf("GetPVEStorageContent: %v", err)
	}
	if len(content) != 1 {
		t.Fatalf("want 1 content item, got %d", len(content))
	}
	if content[0].Verification != "ok" {
		t.Errorf("Verification: want ok, got %q", content[0].Verification)
	}
}

func TestSavePBSReport_FullRoundTrip(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	svc := NewReport(st, time.UTC)
	historyValue := 42.5

	req := &domain.PBSReportRequest{
		Hostname:      "pbs-01",
		IPAddress:     "10.0.0.2",
		ClientVersion: "1.0",
		PBSInformation: domain.PBSInformation{
			Data: []domain.PBSDatastorePayload{
				{
					Store: "datastore1",
					Total: 500,
					Used:  200,
					Avail: 300,
					History: []*float64{
						&historyValue,
						nil,
					},
					Groups: []domain.PBSGroupPayload{{
						BackupType: "vm", BackupID: "100", BackupCount: 2, LastBackup: 123, VerificationState: "ok",
					}},
					GCStatus: &domain.GCStatusPayload{
						DiskBytes: 1024,
						UPID:      "upid-test",
					},
				},
			},
			Tasks: []domain.PBSTaskPayload{{
				TaskType: "sync", JobID: "home-sync", Remote: "casa", RemoteStore: "synology", Store: "local", Status: "OK", EndTime: 123,
			}},
		},
	}

	if err := svc.SavePBSReport(ctx, req); err != nil {
		t.Fatalf("SavePBSReport: %v", err)
	}

	servers, err := st.ListPBSServers(ctx)
	if err != nil {
		t.Fatalf("ListPBSServers: %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("want 1 server, got %d", len(servers))
	}

	rep, err := st.GetLatestPBSReport(ctx, servers[0].ID)
	if err != nil {
		t.Fatalf("GetLatestPBSReport: %v", err)
	}

	stores, err := st.GetPBSStoresForReport(ctx, rep.ID)
	if err != nil {
		t.Fatalf("GetPBSStoresForReport: %v", err)
	}
	if len(stores) != 1 {
		t.Fatalf("want 1 store, got %d", len(stores))
	}
	if stores[0].Store != "datastore1" {
		t.Errorf("Store: want datastore1, got %q", stores[0].Store)
	}
	if stores[0].Total != 500 {
		t.Errorf("Total: want 500, got %d", stores[0].Total)
	}
	history, err := st.GetPBSHistory(ctx, stores[0].ID)
	if err != nil || len(history) != 2 || history[0] == nil || *history[0] != historyValue || history[1] != nil {
		t.Fatalf("unexpected PBS history: %#v, err=%v", history, err)
	}
	snapshots, err := st.GetPBSSnapshotsForStore(ctx, stores[0].ID)
	if err != nil || len(snapshots) != 1 || snapshots[0].BackupID != "100" {
		t.Fatalf("unexpected PBS snapshots: %#v, err=%v", snapshots, err)
	}

	gc, err := st.GetPBSGCStatus(ctx, stores[0].ID)
	if err != nil {
		t.Fatalf("GetPBSGCStatus: %v", err)
	}
	if gc == nil {
		t.Fatal("want GC status, got nil")
	}
	if gc.UPID != "upid-test" {
		t.Errorf("UPID: want upid-test, got %q", gc.UPID)
	}
	tasks, err := st.GetPBSTasksForReport(ctx, rep.ID)
	if err != nil {
		t.Fatalf("GetPBSTasksForReport: %v", err)
	}
	if len(tasks) != 1 || tasks[0].Remote != "casa" || tasks[0].Status != "OK" {
		t.Fatalf("unexpected PBS tasks: %#v", tasks)
	}
}

func TestSaveReportsAreIdempotent(t *testing.T) {
	ctx := context.Background()

	t.Run("pve", func(t *testing.T) {
		db, st := openTestStore(t)
		svc := NewReport(st, time.UTC)
		req := &domain.PVEReportRequest{
			ReportID: "pve-report-1", Hostname: "pve-01",
			Storages: []domain.StoragePayload{{Storage: "local"}},
		}
		if err := svc.SavePVEReport(ctx, req); err != nil {
			t.Fatal(err)
		}
		if err := svc.SavePVEReport(ctx, req); err != nil {
			t.Fatal(err)
		}
		assertReportRowCount(t, db, "pve_reports", 1)
		assertReportRowCount(t, db, "pve_storages", 1)
	})

	t.Run("pbs", func(t *testing.T) {
		db, st := openTestStore(t)
		svc := NewReport(st, time.UTC)
		req := &domain.PBSReportRequest{
			ReportID: "pbs-report-1", Hostname: "pbs-01",
			PBSInformation: domain.PBSInformation{Data: []domain.PBSDatastorePayload{{Store: "backup"}}},
		}
		if err := svc.SavePBSReport(ctx, req); err != nil {
			t.Fatal(err)
		}
		if err := svc.SavePBSReport(ctx, req); err != nil {
			t.Fatal(err)
		}
		assertReportRowCount(t, db, "pbs_reports", 1)
		assertReportRowCount(t, db, "pbs_stores", 1)
	})

	t.Run("windows", func(t *testing.T) {
		db, st := openTestStore(t)
		svc := NewReport(st, time.UTC)
		req := &domain.WindowsReportRequest{
			ReportID: "windows-report-1", Hostname: "windows-01",
			Disks: []domain.WindowsDiskPayload{{Name: "C:", Total: 100, Free: 100}},
		}
		if err := svc.SaveWindowsReportForAPIKey(ctx, req, 0); err != nil {
			t.Fatal(err)
		}
		if err := svc.SaveWindowsReportForAPIKey(ctx, req, 0); err != nil {
			t.Fatal(err)
		}
		assertReportRowCount(t, db, "windows_reports", 1)
		assertReportRowCount(t, db, "windows_disks", 1)
	})
}

func TestPVEAndWindowsReportsRollbackOnChildFailure(t *testing.T) {
	ctx := context.Background()

	t.Run("pve", func(t *testing.T) {
		db, st := openTestStore(t)
		if _, err := db.Exec(`CREATE TRIGGER fail_pve_info BEFORE INSERT ON pve_storage_info BEGIN SELECT RAISE(ABORT, 'forced'); END`); err != nil {
			t.Fatal(err)
		}
		svc := NewReport(st, time.UTC)
		err := svc.SavePVEReport(ctx, &domain.PVEReportRequest{
			ReportID: "pve-failure", Hostname: "pve-01",
			Storages: []domain.StoragePayload{{Storage: "local", StorageInfo: []domain.StorageInfoPayload{{Total: 100}}}},
		})
		if err == nil {
			t.Fatal("expected forced insert failure")
		}
		assertReportRowCount(t, db, "pve_reports", 0)
		assertReportRowCount(t, db, "pve_storages", 0)
	})

	t.Run("windows", func(t *testing.T) {
		db, st := openTestStore(t)
		if _, err := db.Exec(`CREATE TRIGGER fail_windows_disk BEFORE INSERT ON windows_disks BEGIN SELECT RAISE(ABORT, 'forced'); END`); err != nil {
			t.Fatal(err)
		}
		svc := NewReport(st, time.UTC)
		err := svc.SaveWindowsReportForAPIKey(ctx, &domain.WindowsReportRequest{
			ReportID: "windows-failure", Hostname: "windows-01",
			Disks: []domain.WindowsDiskPayload{{Name: "C:", Total: 100}},
		}, 0)
		if err == nil {
			t.Fatal("expected forced insert failure")
		}
		assertReportRowCount(t, db, "windows_reports", 0)
		assertReportRowCount(t, db, "windows_disks", 0)
	})
}

func assertReportRowCount(t *testing.T, db interface {
	QueryRow(query string, args ...any) *sql.Row
}, table string, want int) {
	t.Helper()
	var got int
	if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s rows: got %d, want %d", table, got, want)
	}
}

func TestBuildPVEServerResponse_NoReport(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	svc := NewReport(st, time.UTC)

	serverID, err := st.UpsertPVEServer(ctx, "pve-node", "10.0.0.1", "", "1.0", "")
	if err != nil {
		t.Fatalf("upsert server: %v", err)
	}

	resp := svc.BuildPVEServerResponse(ctx, domain.PVEServer{ID: serverID, Name: "pve-node", IP: "10.0.0.1"})

	if !resp.IsStale {
		t.Error("want IsStale=true for server with no reports")
	}
	if resp.StaleReason == "" {
		t.Error("want non-empty StaleReason")
	}
	if resp.LastReport != nil {
		t.Error("want LastReport=nil")
	}
}

func TestBuildPVEServerResponse_StaleReport(t *testing.T) {
	ctx := context.Background()
	db, st := openTestStore(t)
	svc := NewReport(st, time.UTC)

	serverID, _ := st.UpsertPVEServer(ctx, "pve-node", "10.0.0.1", "", "1.0", "")
	reportID, _ := st.InsertPVEReport(ctx, serverID, nil)

	yesterday := time.Now().Add(-25 * time.Hour)
	if _, err := db.Exec("UPDATE pve_reports SET reported_at = ? WHERE id = ?", yesterday, reportID); err != nil {
		t.Fatalf("backdate report: %v", err)
	}

	resp := svc.BuildPVEServerResponse(ctx, domain.PVEServer{ID: serverID, Name: "pve-node", IP: "10.0.0.1"})

	if !resp.IsStale {
		t.Error("want IsStale=true for yesterday's report")
	}
	if resp.StaleReason != "No se ha recibido el reporte de hoy" {
		t.Errorf("StaleReason: want 'No se ha recibido el reporte de hoy', got %q", resp.StaleReason)
	}
	if resp.LastReport == nil {
		t.Error("want LastReport to be set")
	}
}
