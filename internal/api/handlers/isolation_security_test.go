package handlers_test

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"testing"

	"probakgo/internal/domain"
)

func TestDuplicateWindowsHostnameKeepsHeartbeatsIsolated(t *testing.T) {
	ts := newTestServer(t)
	ctx := context.Background()
	k1, err := ts.store.CreateAPIKey(ctx, "first", "duplicate", "")
	if err != nil {
		t.Fatal(err)
	}
	k2, err := ts.store.CreateAPIKey(ctx, "second", "duplicate", "")
	if err != nil {
		t.Fatal(err)
	}
	id1, err := ts.store.UpsertWindowsServerForAPIKey(ctx, k1.ID, "duplicate", "", "", "", "machine-first")
	if err != nil {
		t.Fatal(err)
	}
	id2, err := ts.store.UpsertWindowsServerForAPIKey(ctx, k2.ID, "duplicate", "", "", "", "machine-second")
	if err != nil {
		t.Fatal(err)
	}
	rr := ts.doJSONWithMachine(t, "POST", "/report/windows", k2.Key, "machine-second", domain.WindowsReportRequest{Hostname: "duplicate", MachineID: "machine-second", IPAddress: "192.0.2.2"})
	if rr.Code != http.StatusOK {
		t.Fatalf("report failed: %d %s", rr.Code, rr.Body.String())
	}
	hb, err := ts.store.GetServerHeartbeat(ctx, "windows", id1)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected no other heartbeat, got %v", err)
	}
	if hb != nil {
		t.Fatalf("other server heartbeat changed: %+v", hb)
	}
	hb, err = ts.store.GetServerHeartbeat(ctx, "windows", id2)
	if err != nil || hb == nil || hb.MachineID != "machine-second" {
		t.Fatalf("reporting server heartbeat missing: %+v, %v", hb, err)
	}
}

func TestDuplicatePVEHostnameKeepsBackupConfigIsolated(t *testing.T) {
	ts := newTestServer(t)
	ctx := context.Background()
	k1, err := ts.store.CreateAPIKey(ctx, "first", "duplicate", "")
	if err != nil {
		t.Fatal(err)
	}
	k2, err := ts.store.CreateAPIKey(ctx, "second", "duplicate", "")
	if err != nil {
		t.Fatal(err)
	}
	id1, err := ts.store.UpsertPVEServerForAPIKey(ctx, k1.ID, "duplicate", "", "", "", "machine-first")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ts.store.CreateVMBackupConfigForServer(ctx, "pve", id1, "duplicate", domain.CreateVMBackupConfigRequest{VMID: "100", VMName: "PRIVATE-FIRST-SERVER-CONFIG"}); err != nil {
		t.Fatal(err)
	}
	rr := ts.doJSONWithMachine(t, "GET", "/backup-config/pve/duplicate", k2.Key, "machine-second", nil)
	exposed := strings.Contains(rr.Body.String(), "PRIVATE-FIRST-SERVER-CONFIG")
	t.Logf("second API key read first API key's backup configuration: HTTP=%d exposed=%v", rr.Code, exposed)
	if rr.Code != http.StatusOK || exposed {
		t.Fatal("API key read another server configuration")
	}
}
