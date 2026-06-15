package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDiscoverPVEVMsListsQEMUAndLXC(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/nodes/test-node/qemu", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"data": []any{
				map[string]any{"vmid": float64(101), "name": "web"},
				map[string]any{"vmid": float64(100), "name": "db"},
			},
		})
	})
	mux.HandleFunc("/api2/json/nodes/test-node/lxc", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"data": []any{
				map[string]any{"vmid": float64(200), "name": "proxy"},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	got, err := newTestPVEClient(srv).discoverPVEVMs()
	if err != nil {
		t.Fatalf("discoverPVEVMs: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("VMs: got %d, want 3", len(got))
	}
	want := []discoveredVM{
		{VMID: "100", Name: "db"},
		{VMID: "101", Name: "web"},
		{VMID: "200", Name: "proxy"},
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("VM[%d]: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestSyncBackupConfigCreatesMissingAndUpdatesUnscheduledVMs(t *testing.T) {
	var created []map[string]any
	var updated []map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/api/backup-config/pve/pve-01", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer pbk-test" {
			t.Fatalf("Authorization: got %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("X-Machine-ID") != "machine-123" {
			t.Fatalf("X-Machine-ID: got %q", r.Header.Get("X-Machine-ID"))
		}
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
				"server": "pve-01",
				"configs": []any{
					map[string]any{"vm_id": "100", "vm_name": "existing"},
					map[string]any{"vm_id": "101", "vm_name": "scheduled", "monday": true},
				},
			})
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("/api/backup-config/pve/pve-01/vms/100", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.NotFound(w, r)
			return
		}
		var body map[string]any
		if r.Header.Get("X-Machine-ID") != "machine-123" {
			t.Fatalf("X-Machine-ID on update: got %q", r.Header.Get("X-Machine-ID"))
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode update body: %v", err)
		}
		updated = append(updated, body)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"status": "updated"}) //nolint:errcheck
	})
	mux.HandleFunc("/api/backup-config/pve/pve-01/vms", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		var body map[string]any
		if r.Header.Get("X-Machine-ID") != "machine-123" {
			t.Fatalf("X-Machine-ID on create: got %q", r.Header.Get("X-Machine-ID"))
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode create body: %v", err)
		}
		created = append(created, body)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"id": len(created)}) //nolint:errcheck
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	count, updatedCount, skipped, err := syncBackupConfig(&Config{APIURL: srv.URL, APIKey: "pbk-test"}, "pve-01", "machine-123", []discoveredVM{
		{VMID: "100", Name: "existing"},
		{VMID: "101", Name: "scheduled"},
		{VMID: "200", Name: "proxy"},
	})
	if err != nil {
		t.Fatalf("syncBackupConfig: %v", err)
	}
	if count != 1 || updatedCount != 1 || skipped != 1 {
		t.Fatalf("created/updated/skipped: got %d/%d/%d, want 1/1/1", count, updatedCount, skipped)
	}
	if len(updated) != 1 {
		t.Fatalf("updated requests: got %d, want 1", len(updated))
	}
	if updated[0]["vm_id"] != "100" || updated[0]["vm_name"] != "existing" || updated[0]["monday"] != true || updated[0]["friday"] != true {
		t.Fatalf("updated body: %+v", updated[0])
	}
	if len(created) != 1 {
		t.Fatalf("created requests: got %d, want 1", len(created))
	}
	if created[0]["vm_id"] != "200" || created[0]["vm_name"] != "proxy" || created[0]["monday"] != true || created[0]["friday"] != true {
		t.Fatalf("created body: %+v", created[0])
	}
}

func TestDiscoverPVEBackupSchedulesUsesClusterBackupJobs(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/cluster/backup", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"data": []any{
				map[string]any{
					"enabled":  float64(1),
					"schedule": "mon..sat 22:15",
					"vmid":     "400,500,600,700,800,100",
				},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	schedules, err := newTestPVEClient(srv).discoverPVEBackupSchedules([]discoveredVM{
		{VMID: "100", Name: "adguard"},
		{VMID: "400", Name: "debian"},
	})
	if err != nil {
		t.Fatalf("discoverPVEBackupSchedules: %v", err)
	}
	got := schedules["100"].Days
	if !got.Monday || !got.Friday || !got.Saturday || got.Sunday {
		t.Fatalf("VM 100 days: got %+v, want mon..sat", got)
	}
}

func TestDiscoverPVEBackupSchedulesPrefersWeekdayNightJobAndAddsWeekend(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/cluster/backup", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"data": []any{
				map[string]any{"enabled": float64(1), "schedule": "mon..fri 12:00", "vmid": "101"},
				map[string]any{"enabled": float64(1), "schedule": "mon..fri 22:30", "vmid": "101"},
				map[string]any{"enabled": float64(1), "schedule": "sat 23:00", "vmid": "101"},
				map[string]any{"enabled": float64(0), "schedule": "sun 23:00", "vmid": "101"},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	schedules, err := newTestPVEClient(srv).discoverPVEBackupSchedules([]discoveredVM{{VMID: "101", Name: "web"}})
	if err != nil {
		t.Fatalf("discoverPVEBackupSchedules: %v", err)
	}
	got := schedules["101"]
	if got.StartMin != 22*60+30 {
		t.Fatalf("StartMin: got %d, want 1350", got.StartMin)
	}
	if !got.Days.Monday || !got.Days.Friday || !got.Days.Saturday || got.Days.Sunday {
		t.Fatalf("days: got %+v, want weekdays plus saturday only", got.Days)
	}
}

func TestSyncBackupConfigUsesDiscoveredSchedule(t *testing.T) {
	var created []map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/api/backup-config/pve/pve-01", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"server": "pve-01", "configs": []any{}}) //nolint:errcheck
	})
	mux.HandleFunc("/api/backup-config/pve/pve-01/vms", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode create body: %v", err)
		}
		created = append(created, body)
		w.WriteHeader(http.StatusCreated)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	createdCount, updated, skipped, err := syncBackupConfig(
		&Config{APIURL: srv.URL, APIKey: "pbk-test"},
		"pve-01",
		"machine-123",
		[]discoveredVM{{VMID: "100", Name: "adguard"}},
		map[string]pveBackupSchedule{
			"100": {Days: backupDays{Monday: true, Tuesday: true, Wednesday: true, Thursday: true, Friday: true, Saturday: true}},
		},
	)
	if err != nil {
		t.Fatalf("syncBackupConfig: %v", err)
	}
	if createdCount != 1 || updated != 0 || skipped != 0 {
		t.Fatalf("created/updated/skipped: got %d/%d/%d, want 1/0/0", createdCount, updated, skipped)
	}
	if len(created) != 1 || created[0]["saturday"] != true || created[0]["sunday"] == true {
		t.Fatalf("created body: %+v", created)
	}
}
