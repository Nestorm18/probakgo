package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetBackupConfig_Empty(t *testing.T) {
	ctx := context.Background()
	ts := newTestServer(t)
	k, _ := ts.store.CreateAPIKey(ctx, "client", "", "")

	req := httptest.NewRequest(http.MethodGet, "/backup-config/pve/pve-01", nil)
	req.Header.Set("Authorization", "Bearer "+k.Key)
	req.Header.Set("X-Machine-ID", "machine-1")

	rr := httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestGetBackupConfig_PreservesPVEServerMetadata(t *testing.T) {
	ctx := context.Background()
	ts := newTestServer(t)
	k, _ := ts.store.CreateAPIKey(ctx, "client", "", "")
	serverID, err := ts.store.UpsertPVEServerForAPIKey(
		ctx,
		k.ID,
		"pve-01",
		"192.0.2.10",
		"198.51.100.10",
		"0.0.194",
		"machine-1",
	)
	if err != nil {
		t.Fatalf("create PVE server: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/backup-config/pve/pve-01", nil)
	req.Header.Set("Authorization", "Bearer "+k.Key)
	req.Header.Set("X-Machine-ID", "machine-1")

	rr := httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	server, err := ts.store.GetPVEServer(ctx, serverID)
	if err != nil {
		t.Fatalf("get PVE server: %v", err)
	}
	if server.IP != "192.0.2.10" || server.PublicIP != "198.51.100.10" || server.ClientVersion != "0.0.194" {
		t.Fatalf(
			"backup config request changed PVE metadata: ip=%q public_ip=%q client_version=%q",
			server.IP,
			server.PublicIP,
			server.ClientVersion,
		)
	}
}

func TestCreateVMConfig_HappyPath(t *testing.T) {
	ctx := context.Background()
	ts := newTestServer(t)
	k, _ := ts.store.CreateAPIKey(ctx, "client", "", "")

	body := `{"vm_id":"100","vm_name":"web","monday":true}`
	req := httptest.NewRequest(http.MethodPost, "/backup-config/pve/pve-01/vms", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+k.Key)
	req.Header.Set("X-Machine-ID", "machine-1")

	rr := httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("want 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCreateVMConfig_MissingVMID(t *testing.T) {
	ctx := context.Background()
	ts := newTestServer(t)
	k, _ := ts.store.CreateAPIKey(ctx, "client", "", "")

	body := `{"vm_name":"web"}`
	req := httptest.NewRequest(http.MethodPost, "/backup-config/pve/pve-01/vms", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+k.Key)
	req.Header.Set("X-Machine-ID", "machine-1")

	rr := httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
