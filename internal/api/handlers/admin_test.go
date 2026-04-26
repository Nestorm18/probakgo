package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateAPIKey_HappyPath(t *testing.T) {
	ts := newTestServer(t)
	adminKey, _ := ts.store.CreateAPIKey("myadmin", "admin", "")

	body := `{"name":"client-key","key_type":"server"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/api-keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminKey.Key)

	rr := httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["key"] == nil || resp["key"] == "" {
		t.Errorf("want key in response, got %v", resp)
	}
}

func TestCreateAPIKey_MissingFields(t *testing.T) {
	ts := newTestServer(t)
	adminKey, _ := ts.store.CreateAPIKey("myadmin", "admin", "")

	body := `{"name":""}`
	req := httptest.NewRequest(http.MethodPost, "/admin/api-keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminKey.Key)

	rr := httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCreateAPIKey_RequiresAdminKey(t *testing.T) {
	ts := newTestServer(t)
	serverKey, _ := ts.store.CreateAPIKey("client", "server", "")

	body := `{"name":"k","key_type":"server"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/api-keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+serverKey.Key)

	rr := httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestListAPIKeys(t *testing.T) {
	ts := newTestServer(t)
	adminKey, _ := ts.store.CreateAPIKey("myadmin", "admin", "")
	_, _ = ts.store.CreateAPIKey("client-1", "server", "")

	req := httptest.NewRequest(http.MethodGet, "/admin/api-keys", nil)
	req.Header.Set("Authorization", "Bearer "+adminKey.Key)

	rr := httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	keys, ok := resp["api_keys"].([]any)
	if !ok {
		t.Fatal("want api_keys array in response")
	}
	if len(keys) != 2 {
		t.Errorf("want 2 keys, got %d", len(keys))
	}
}

func TestDeleteAPIKey(t *testing.T) {
	ts := newTestServer(t)
	adminKey, _ := ts.store.CreateAPIKey("myadmin", "admin", "")
	target, _ := ts.store.CreateAPIKey("to-delete", "server", "")

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/admin/api-keys/%d", target.ID), nil)
	req.Header.Set("Authorization", "Bearer "+adminKey.Key)

	rr := httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
}
