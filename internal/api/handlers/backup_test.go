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

	rr := httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
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

	rr := httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
