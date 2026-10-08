package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAPIReadsDoNotBindKeyOrCreateServers(t *testing.T) {
	ctx := context.Background()
	ts := newTestServer(t)
	k, err := ts.store.CreateAPIKey(ctx, "client", "", "")
	if err != nil {
		t.Fatal(err)
	}
	res, err := ts.db.ExecContext(ctx, `INSERT INTO pve_servers (name, machine_id, ip, public_ip, client_version) VALUES ('legacy-pve', 'machine-1', '', '', '')`)
	if err != nil {
		t.Fatal(err)
	}
	legacyID, _ := res.LastInsertId()

	if rr := ts.doJSON(t, http.MethodGet, "/backup-config/pve/new-pve", k.Key, nil); rr.Code != http.StatusOK {
		t.Fatalf("backup config: want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if rr := ts.doJSON(t, http.MethodGet, fmt.Sprintf("/servers/pve/%d/reports", legacyID), k.Key, nil); rr.Code != http.StatusForbidden {
		t.Fatalf("legacy reports with an unbound key: want 403, got %d: %s", rr.Code, rr.Body.String())
	}

	var servers int
	if err := ts.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pve_servers WHERE name = 'new-pve' OR api_key_id IS NOT NULL`).Scan(&servers); err != nil {
		t.Fatal(err)
	}
	if servers != 0 {
		t.Fatalf("a read created or bound %d PVE servers", servers)
	}
	key, err := ts.store.GetAPIKey(ctx, k.ID)
	if err != nil {
		t.Fatal(err)
	}
	if key.ServerName != "" {
		t.Fatalf("a read bound the key to server %q", key.ServerName)
	}
}

func TestBackupConfigWritesRejectInvalidVMIDAndJSON(t *testing.T) {
	ctx := context.Background()
	ts := newTestServer(t)
	k, err := ts.store.CreateAPIKey(ctx, "client", "", "")
	if err != nil {
		t.Fatal(err)
	}
	base := "/backup-config/pve/pve-01"

	for _, vmid := range []string{"", "abc", "99", "0100", "1000000000"} {
		rr := ts.doJSON(t, http.MethodPost, base+"/vms", k.Key, map[string]string{"vm_id": vmid})
		if rr.Code != http.StatusBadRequest {
			t.Errorf("create with vm_id %q: want 400, got %d", vmid, rr.Code)
		}
	}
	for _, vmid := range []string{"abc", "99"} {
		rr := ts.doJSON(t, http.MethodDelete, base+"/vms/"+vmid, k.Key, nil)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("delete vmid %q: want 400, got %d", vmid, rr.Code)
		}
	}

	req := httptest.NewRequest(http.MethodPut, base, strings.NewReader(`{"has_vms":true}{"has_vms":false}`))
	req.Header.Set("Authorization", "Bearer "+k.Key)
	req.Header.Set("X-Machine-ID", "machine-1")
	rr := httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("inventory with two JSON values: want 400, got %d", rr.Code)
	}

	if rr := ts.doJSON(t, http.MethodPost, base+"/vms", k.Key, map[string]string{"vm_id": "101"}); rr.Code != http.StatusCreated {
		t.Fatalf("create with a valid vm_id: want 201, got %d: %s", rr.Code, rr.Body.String())
	}
}
