package service

import (
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

	if err := svc.SavePVEReport(req); err != nil {
		t.Fatalf("SavePVEReport: %v", err)
	}

	servers, err := st.ListPVEServers()
	if err != nil {
		t.Fatalf("ListPVEServers: %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("want 1 server, got %d", len(servers))
	}

	rep, err := st.GetLatestPVEReport(servers[0].ID)
	if err != nil {
		t.Fatalf("GetLatestPVEReport: %v", err)
	}
	if rep.BackupStatus != "OK" {
		t.Errorf("BackupStatus: want OK, got %q", rep.BackupStatus)
	}
	if rep.BackupDuration != 1000 {
		t.Errorf("BackupDuration: want 1000, got %d", rep.BackupDuration)
	}

	storages, err := st.GetPVEStoragesForReport(rep.ID)
	if err != nil {
		t.Fatalf("GetPVEStoragesForReport: %v", err)
	}
	if len(storages) != 1 {
		t.Fatalf("want 1 storage, got %d", len(storages))
	}

	content, err := st.GetPVEStorageContent(storages[0].ID)
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
	_, st := openTestStore(t)
	svc := NewReport(st, time.UTC)

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
					GCStatus: &domain.GCStatusPayload{
						DiskBytes: 1024,
						UPID:      "upid-test",
					},
				},
			},
		},
	}

	if err := svc.SavePBSReport(req); err != nil {
		t.Fatalf("SavePBSReport: %v", err)
	}

	servers, err := st.ListPBSServers()
	if err != nil {
		t.Fatalf("ListPBSServers: %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("want 1 server, got %d", len(servers))
	}

	rep, err := st.GetLatestPBSReport(servers[0].ID)
	if err != nil {
		t.Fatalf("GetLatestPBSReport: %v", err)
	}

	stores, err := st.GetPBSStoresForReport(rep.ID)
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

	gc, err := st.GetPBSGCStatus(stores[0].ID)
	if err != nil {
		t.Fatalf("GetPBSGCStatus: %v", err)
	}
	if gc == nil {
		t.Fatal("want GC status, got nil")
	}
	if gc.UPID != "upid-test" {
		t.Errorf("UPID: want upid-test, got %q", gc.UPID)
	}
}

func TestBuildPVEServerResponse_NoReport(t *testing.T) {
	_, st := openTestStore(t)
	svc := NewReport(st, time.UTC)

	serverID, err := st.UpsertPVEServer("pve-node", "10.0.0.1", "", "1.0", "")
	if err != nil {
		t.Fatalf("upsert server: %v", err)
	}

	resp := svc.BuildPVEServerResponse(domain.PVEServer{ID: serverID, Name: "pve-node", IP: "10.0.0.1"})

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
	db, st := openTestStore(t)
	svc := NewReport(st, time.UTC)

	serverID, _ := st.UpsertPVEServer("pve-node", "10.0.0.1", "", "1.0", "")
	reportID, _ := st.InsertPVEReport(serverID, nil)

	yesterday := time.Now().Add(-25 * time.Hour)
	if _, err := db.Exec("UPDATE pve_reports SET reported_at = ? WHERE id = ?", yesterday, reportID); err != nil {
		t.Fatalf("backdate report: %v", err)
	}

	resp := svc.BuildPVEServerResponse(domain.PVEServer{ID: serverID, Name: "pve-node", IP: "10.0.0.1"})

	if !resp.IsStale {
		t.Error("want IsStale=true for yesterday's report")
	}
	if resp.StaleReason != "no report received today" {
		t.Errorf("StaleReason: want 'no report received today', got %q", resp.StaleReason)
	}
	if resp.LastReport == nil {
		t.Error("want LastReport to be set")
	}
}
