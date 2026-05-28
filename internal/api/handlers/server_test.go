package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListPVEServers_Empty(t *testing.T) {
	ctx := context.Background()
	ts := newTestServer(t)
	k, _ := ts.store.CreateAPIKey(ctx, "client", "", "")

	req := httptest.NewRequest(http.MethodGet, "/servers/pve", nil)
	req.Header.Set("Authorization", "Bearer "+k.Key)
	req.Header.Set("X-Machine-ID", "machine-1")

	rr := httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	servers, ok := resp["servers"].([]any)
	if !ok {
		t.Fatal("want servers array in response")
	}
	if len(servers) != 0 {
		t.Errorf("want empty servers list for unbound key, got %d", len(servers))
	}
}

func TestListPVEServers_WithServer(t *testing.T) {
	ctx := context.Background()
	ts := newTestServer(t)
	k, _ := ts.store.CreateAPIKey(ctx, "client", "pve-01", "")
	ts.store.UpsertPVEServer(ctx, "pve-01", "10.0.0.1", "", "1.0", "")

	req := httptest.NewRequest(http.MethodGet, "/servers/pve", nil)
	req.Header.Set("Authorization", "Bearer "+k.Key)
	req.Header.Set("X-Machine-ID", "machine-1")

	rr := httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	servers, ok := resp["servers"].([]any)
	if !ok || len(servers) != 1 {
		t.Fatalf("want 1 server, got %v", resp["servers"])
	}
	sv := servers[0].(map[string]any)
	if sv["is_stale"] != true {
		t.Error("want is_stale=true for server without reports")
	}
}

func TestListPVEReports_NotFound(t *testing.T) {
	ctx := context.Background()
	ts := newTestServer(t)
	k, _ := ts.store.CreateAPIKey(ctx, "client", "", "")

	req := httptest.NewRequest(http.MethodGet, "/servers/pve/9999/reports", nil)
	req.Header.Set("Authorization", "Bearer "+k.Key)
	req.Header.Set("X-Machine-ID", "machine-1")

	rr := httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestListPVEReports_HappyPath(t *testing.T) {
	ctx := context.Background()
	ts := newTestServer(t)
	k, _ := ts.store.CreateAPIKey(ctx, "client", "pve-01", "")
	serverID, _ := ts.store.UpsertPVEServer(ctx, "pve-01", "10.0.0.1", "", "1.0", "")
	ts.store.InsertPVEReport(ctx, serverID, nil)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/servers/pve/%d/reports", serverID), nil)
	req.Header.Set("Authorization", "Bearer "+k.Key)
	req.Header.Set("X-Machine-ID", "machine-1")

	rr := httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["server"] == nil {
		t.Error("want server in response")
	}
	reports, ok := resp["reports"].([]any)
	if !ok || len(reports) != 1 {
		t.Errorf("want 1 report, got %v", resp["reports"])
	}
}
