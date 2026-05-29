package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHeartbeatPVE_HappyPath(t *testing.T) {
	ctx := context.Background()
	ts := newTestServer(t)
	k, err := ts.store.CreateAPIKey(ctx, "client", "", "")
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}

	body := `{"hostname":"pve-01","server_type":"pve","ip_address":"10.0.0.1","client_version":"0.0.65","machine_id":"machine-1"}`
	req := httptest.NewRequest(http.MethodPost, "/heartbeat", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+k.Key)
	req.Header.Set("X-Machine-ID", "machine-1")

	rr := httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	sv, err := ts.store.GetPVEServerByName(ctx, "pve-01")
	if err != nil {
		t.Fatalf("GetPVEServerByName: %v", err)
	}
	hb, err := ts.store.GetServerHeartbeat(ctx, "pve", sv.ID)
	if err != nil {
		t.Fatalf("GetServerHeartbeat: %v", err)
	}
	if hb.ClientVersion != "0.0.65" || hb.IP != "10.0.0.1" {
		t.Fatalf("unexpected heartbeat: %#v", hb)
	}
}

func TestHeartbeat_ServerNameMismatch(t *testing.T) {
	ctx := context.Background()
	ts := newTestServer(t)
	k, _ := ts.store.CreateAPIKey(ctx, "client", "pve-01", "")

	body := `{"hostname":"pve-02","server_type":"pve","ip_address":"10.0.0.2","machine_id":"machine-1"}`
	req := httptest.NewRequest(http.MethodPost, "/heartbeat", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+k.Key)
	req.Header.Set("X-Machine-ID", "machine-1")

	rr := httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d: %s", rr.Code, rr.Body.String())
	}
}
