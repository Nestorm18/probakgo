package handlers_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"probakgo/internal/domain"
)

func TestAPIIntegration_PVEReportThenListServersAndReports(t *testing.T) {
	ctx := context.Background()
	ts := newTestServer(t)
	k, err := ts.store.CreateAPIKey(ctx, "client", "", "")
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}

	req := domain.PVEReportRequest{
		Hostname:      "pve-01",
		IPAddress:     "10.0.0.1",
		PublicIP:      "203.0.113.10",
		ClientVersion: "1.2.3",
		LastBackupStatus: &domain.BackupStatus{
			Status:    []byte(`"OK"`),
			StartTime: time.Now().Add(-2 * time.Hour).Unix(),
			EndTime:   time.Now().Add(-1 * time.Hour).Unix(),
			Duration:  3600,
		},
		Storages: []domain.StoragePayload{{
			Storage: "local",
			Path:    "/var/lib/vz",
			Content: "backup",
			Type:    "dir",
			Status:  "available",
			StorageInfo: []domain.StorageInfoPayload{{
				Total: 1000,
				Used:  400,
				Avail: 600,
			}},
			ContentData: []domain.ContentDataPayload{{
				VMID:    100,
				Format:  "vma.zst",
				Size:    1234,
				Content: "backup",
				VolID:   "local:backup/vzdump-qemu-100.vma.zst",
				CTime:   time.Now().Unix(),
			}},
		}},
		BackupTasks: []domain.BackupTaskPayload{{
			VMID:      100,
			VMName:    "web",
			Status:    "OK",
			StartTime: time.Now().Add(-2 * time.Hour).Unix(),
			EndTime:   time.Now().Add(-1 * time.Hour).Unix(),
			Duration:  3600,
			Size:      1234,
			Filename:  "local:backup/vzdump-qemu-100.vma.zst",
		}},
	}

	rr := ts.doJSON(t, http.MethodPost, "/report/pve", k.Key, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /report/pve: want 200, got %d: %s", rr.Code, rr.Body.String())
	}

	rr = ts.doJSON(t, http.MethodGet, "/servers/pve", k.Key, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /servers/pve: want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var serversResp struct {
		Servers []domain.PVEServerResponse `json:"servers"`
	}
	decodeJSON(t, rr, &serversResp)
	if len(serversResp.Servers) != 1 {
		t.Fatalf("servers: got %d, want 1", len(serversResp.Servers))
	}
	server := serversResp.Servers[0]
	if server.Name != "pve-01" || server.IP != "10.0.0.1" || server.BackupStatus != "OK" {
		t.Fatalf("unexpected server response: %+v", server)
	}
	if server.IsStale {
		t.Fatal("reported server should not be stale")
	}

	rr = ts.doJSON(t, http.MethodGet, "/servers/pve/1/reports?limit=1", k.Key, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /servers/pve/1/reports: want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var reportsResp struct {
		Server  domain.PVEServerResponse `json:"server"`
		Reports []domain.PVEReport       `json:"reports"`
	}
	decodeJSON(t, rr, &reportsResp)
	if reportsResp.Server.Name != "pve-01" {
		t.Fatalf("server name: got %q, want pve-01", reportsResp.Server.Name)
	}
	if len(reportsResp.Reports) != 1 {
		t.Fatalf("reports: got %d, want 1", len(reportsResp.Reports))
	}
	if reportsResp.Reports[0].BackupStatus != "OK" {
		t.Fatalf("backup status: got %q, want OK", reportsResp.Reports[0].BackupStatus)
	}
}

func TestAPIIntegration_BackupConfigLifecycle(t *testing.T) {
	ctx := context.Background()
	ts := newTestServer(t)
	k, err := ts.store.CreateAPIKey(ctx, "client", "", "")
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}

	createReq := domain.CreateVMBackupConfigRequest{
		VMID:   "100",
		VMName: "web",
		Monday: true,
	}
	rr := ts.doJSON(t, http.MethodPost, "/backup-config/pve/pve-01/vms", k.Key, createReq)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create config: want 201, got %d: %s", rr.Code, rr.Body.String())
	}

	updateReq := domain.CreateVMBackupConfigRequest{
		VMName:  "web-renamed",
		Tuesday: true,
	}
	rr = ts.doJSON(t, http.MethodPut, "/backup-config/pve/pve-01/vms/100", k.Key, updateReq)
	if rr.Code != http.StatusOK {
		t.Fatalf("update config: want 200, got %d: %s", rr.Code, rr.Body.String())
	}

	rr = ts.doJSON(t, http.MethodPut, "/backup-config/pve/pve-01/vms/100/toggle-exclude", k.Key, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("toggle config: want 200, got %d: %s", rr.Code, rr.Body.String())
	}

	rr = ts.doJSON(t, http.MethodGet, "/backup-config/pve/pve-01", k.Key, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("list config: want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var listResp struct {
		Server  string                          `json:"server"`
		Configs []domain.VMBackupConfigResponse `json:"configs"`
	}
	decodeJSON(t, rr, &listResp)
	if listResp.Server != "pve-01" || len(listResp.Configs) != 1 {
		t.Fatalf("unexpected config list: %+v", listResp)
	}
	cfg := listResp.Configs[0]
	if cfg.VMID != "100" || cfg.VMName != "web-renamed" || !cfg.Tuesday || !cfg.IsExcluded {
		t.Fatalf("unexpected config after update/toggle: %+v", cfg)
	}

	rr = ts.doJSON(t, http.MethodDelete, "/backup-config/pve/pve-01/vms/100", k.Key, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("delete config: want 200, got %d: %s", rr.Code, rr.Body.String())
	}

	rr = ts.doJSON(t, http.MethodGet, "/backup-config/pve/pve-01", k.Key, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("list after delete: want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	decodeJSON(t, rr, &listResp)
	if len(listResp.Configs) != 0 {
		t.Fatalf("configs after delete: got %d, want 0", len(listResp.Configs))
	}
}
