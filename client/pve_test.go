package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newPVEMockServer returns a test server that mimics the Proxmox VE API.
// It exposes a single PBS-type storage with three content items covering
// all verification states: ok, failed, and absent.
func newPVEMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/api2/json/storage", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"data": []any{
				map[string]any{
					"storage": "pbs-store",
					"type":    "pbs",
					"content": "backup",
					"digest":  "abc123",
				},
			},
		})
	})

	mux.HandleFunc("/api2/json/nodes/test-node/storage", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": []any{}}) //nolint:errcheck
	})

	mux.HandleFunc("/api2/json/nodes/test-node/storage/pbs-store/content", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"data": []any{
				map[string]any{
					"vmid": float64(100), "format": "pxar.didx", "size": float64(1073741824),
					"content": "backup", "volid": "pbs-store:backup/vm/100/2025-01-01T08:00:00Z",
					"ctime": float64(1735689600), "subtype": "vm", "notes": "web",
					"verification": map[string]any{"state": "ok"},
				},
				map[string]any{
					"vmid": float64(101), "format": "pxar.didx", "size": float64(536870912),
					"content": "backup", "volid": "pbs-store:backup/vm/101/2025-01-01T08:30:00Z",
					"ctime": float64(1735691400), "subtype": "vm", "notes": "db",
					"verification": map[string]any{"state": "failed"},
				},
				map[string]any{
					"vmid": float64(200), "format": "pxar.didx", "size": float64(268435456),
					"content": "backup", "volid": "pbs-store:backup/ct/200/2025-01-01T09:00:00Z",
					"ctime": float64(1735693200), "subtype": "ct",
					// no verification field → should produce ""
				},
			},
		})
	})

	mux.HandleFunc("/api2/json/nodes/test-node/tasks", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": []any{}}) //nolint:errcheck
	})

	return httptest.NewServer(mux)
}

func newTestPVEClient(srv *httptest.Server) *pveClient {
	cfg := &Config{VerifyTLS: false}
	si := &SysInfo{Hostname: "test-node", cfg: cfg}
	return &pveClient{
		cfg:     cfg,
		si:      si,
		http:    newHTTPClient(cfg),
		baseURL: srv.URL + "/api2/json",
	}
}

func TestGenerateReportVerification(t *testing.T) {
	srv := newPVEMockServer(t)
	defer srv.Close()

	report, err := newTestPVEClient(srv).generateReport()
	if err != nil {
		t.Fatalf("generateReport: %v", err)
	}

	storages, ok := report["storages"].([]map[string]any)
	if !ok || len(storages) != 1 {
		t.Fatalf("expected 1 storage, got %T len=%d", report["storages"], len(storages))
	}

	contents, ok := storages[0]["content_data"].([]any)
	if !ok {
		t.Fatalf("content_data: expected []any, got %T", storages[0]["content_data"])
	}
	if len(contents) != 3 {
		t.Fatalf("expected 3 content items, got %d", len(contents))
	}

	want := []string{"ok", "failed", ""}
	for i, w := range want {
		item := contents[i].(map[string]any)
		got, _ := item["verification"].(string)
		if got != w {
			t.Errorf("content[%d] verification: got %q, want %q", i, got, w)
		}
	}
}

func TestLastBackupStatusEmpty(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/nodes/test-node/tasks", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": []any{}}) //nolint:errcheck
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	bs := newTestPVEClient(srv).lastBackupStatus()

	if bs.OK {
		t.Error("expected OK=false for empty task list")
	}
	if bs.StartTime != -1 || bs.EndTime != -1 || bs.Duration != -1 {
		t.Errorf("expected -1 times, got start=%d end=%d dur=%d", bs.StartTime, bs.EndTime, bs.Duration)
	}
}

func TestLastBackupStatusPicksMostRecent(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/nodes/test-node/tasks", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"data": []any{
				// oldest, succeeded
				map[string]any{"starttime": float64(500), "endtime": float64(900), "status": "OK"},
				// middle, succeeded
				map[string]any{"starttime": float64(1000), "endtime": float64(2000), "status": "OK"},
				// most recent, failed - this one must win
				map[string]any{"starttime": float64(3000), "endtime": float64(5000), "status": "ERROR: disk full"},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	bs := newTestPVEClient(srv).lastBackupStatus()

	if bs.OK {
		t.Error("expected OK=false: most recent task had ERROR status")
	}
	if bs.StartTime != 3000 {
		t.Errorf("StartTime: got %d, want 3000", bs.StartTime)
	}
	if bs.EndTime != 5000 {
		t.Errorf("EndTime: got %d, want 5000", bs.EndTime)
	}
	if bs.Duration != 2000 {
		t.Errorf("Duration: got %d, want 2000", bs.Duration)
	}
}

func TestLastBackupStatusMostRecentOK(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/nodes/test-node/tasks", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"data": []any{
				map[string]any{"starttime": float64(1000), "endtime": float64(2000), "status": "ERROR"},
				map[string]any{"starttime": float64(3000), "endtime": float64(5000), "status": "OK"},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	bs := newTestPVEClient(srv).lastBackupStatus()

	if !bs.OK {
		t.Error("expected OK=true: most recent task was OK")
	}
	if bs.StartTime != 3000 {
		t.Errorf("StartTime: got %d, want 3000", bs.StartTime)
	}
}

func TestBackupJobTasksReconstructsAggregateJobFromFiles(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/nodes/test-node/tasks", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"data": []any{
				map[string]any{
					"id":        "",
					"starttime": float64(10000),
					"endtime":   float64(10200),
					"status":    "OK",
				},
				map[string]any{
					"id":        "101",
					"starttime": float64(100),
					"endtime":   float64(200),
					"status":    "ERROR",
				},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	filesByVMID := map[int64][]backupFile{
		101: {{ctime: 10010, size: 39_000_000, volid: "nas:backup/vzdump-qemu-101.vma.zst"}},
		3100: {{ctime: 10130, size: 3_730_000_000,
			volid: "nas:backup/vzdump-qemu-3100.vma.zst"}},
	}

	tasks := newTestPVEClient(srv).backupJobTasks(
		map[int64]string{101: "mikrotik-routeros-chr", 3100: "plik"},
		filesByVMID,
	)

	if len(tasks) != 2 {
		t.Fatalf("expected 2 reconstructed tasks, got %d", len(tasks))
	}
	if got := tasks[0]["vmid"]; got != int64(101) {
		t.Errorf("first vmid: got %v, want 101", got)
	}
	if got := tasks[1]["vmid"]; got != int64(3100) {
		t.Errorf("second vmid: got %v, want 3100", got)
	}
	for _, task := range tasks {
		if got := task["status"]; got != "OK" {
			t.Errorf("status: got %v, want OK", got)
		}
		if got := task["duration"]; got != int64(0) {
			t.Errorf("duration without per-VM log: got %v, want 0", got)
		}
	}
}

func TestBackupJobTasksUsesAggregateLogDurations(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/nodes/test-node/tasks", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"data": []any{
				map[string]any{
					"id":        "",
					"upid":      "UPID:test-node:000123:ABCDEF:664BEEF0:vzdump::root@pam:",
					"starttime": float64(10000),
					"endtime":   float64(11000),
					"status":    "OK",
				},
			},
		})
	})
	mux.HandleFunc("/api2/json/nodes/test-node/tasks/", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"data": []any{
				map[string]any{"n": float64(1), "t": "INFO: Starting Backup of VM 101 (qemu)"},
				map[string]any{"n": float64(2), "t": "INFO: Finished Backup of VM 101 (00:02:15)"},
				map[string]any{"n": float64(3), "t": "INFO: Starting Backup of VM 3100 (qemu)"},
				map[string]any{"n": float64(4), "t": "INFO: Finished Backup of VM 3100 (01:00:05)"},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	filesByVMID := map[int64][]backupFile{
		101:  {{ctime: 10010, size: 39_000_000, volid: "nas:backup/vzdump-qemu-101.vma.zst"}},
		3100: {{ctime: 10130, size: 3_730_000_000, volid: "nas:backup/vzdump-qemu-3100.vma.zst"}},
	}

	tasks := newTestPVEClient(srv).backupJobTasks(
		map[int64]string{101: "mikrotik-routeros-chr", 3100: "plik"},
		filesByVMID,
	)

	if len(tasks) != 2 {
		t.Fatalf("expected 2 reconstructed tasks, got %d", len(tasks))
	}
	if got := tasks[0]["duration"]; got != int64(135) {
		t.Errorf("vm 101 duration: got %v, want 135", got)
	}
	if got := tasks[0]["starttime"]; got != int64(10010) {
		t.Errorf("vm 101 starttime: got %v, want 10010", got)
	}
	if got := tasks[0]["endtime"]; got != int64(10145) {
		t.Errorf("vm 101 endtime: got %v, want 10145", got)
	}
	if got := tasks[1]["duration"]; got != int64(3605) {
		t.Errorf("vm 3100 duration: got %v, want 3605", got)
	}
}

func TestParseBackupDurations(t *testing.T) {
	got := parseBackupDurations([]string{
		"INFO: Finished Backup of VM 101 (00:02:15)",
		"INFO: Finished Backup of VM 202 (12:34)",
		"INFO: Finished Backup of CT 303 in 00:01:02",
		"INFO: Backup of VM 404 finished (00:03:04)",
		"INFO: unrelated",
	})

	if got[101] != 135 {
		t.Errorf("VM 101: got %d, want 135", got[101])
	}
	if got[202] != 754 {
		t.Errorf("VM 202: got %d, want 754", got[202])
	}
	if got[303] != 62 {
		t.Errorf("CT 303: got %d, want 62", got[303])
	}
	if got[404] != 184 {
		t.Errorf("VM 404: got %d, want 184", got[404])
	}
}

func TestGenerateReportStorageOfflineOnContentError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/storage", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"data": []any{
				map[string]any{"storage": "nas-offline", "type": "nfs", "content": "backup"},
			},
		})
	})
	mux.HandleFunc("/api2/json/nodes/test-node/storage", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": []any{}}) //nolint:errcheck
	})
	mux.HandleFunc("/api2/json/nodes/test-node/tasks", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": []any{}}) //nolint:errcheck
	})
	// No handler for content → 404 → get() returns an error → status should be "offline"
	srv := httptest.NewServer(mux)
	defer srv.Close()

	report, err := newTestPVEClient(srv).generateReport()
	if err != nil {
		t.Fatalf("generateReport: %v", err)
	}
	storages := report["storages"].([]map[string]any)
	if got := storages[0]["status"]; got != "offline" {
		t.Errorf("status: got %v, want offline", got)
	}
}
