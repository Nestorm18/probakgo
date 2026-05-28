package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestPBSClient(srv *httptest.Server) *pbsClient {
	cfg := &Config{VerifyTLS: false}
	si := &SysInfo{Hostname: "pbs-test", cfg: cfg}
	return &pbsClient{
		cfg:     cfg,
		si:      si,
		http:    newHTTPClient(cfg),
		baseURL: srv.URL + "/api2/json",
	}
}

func TestPBSGet(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/version", func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			t.Error("missing Authorization header")
		}
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"version": "3.1"}}) //nolint:errcheck
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := &Config{VerifyTLS: false, ProxmoxToken: "root@pam!probakgo", ProxmoxSecret: "secret"}
	si := &SysInfo{Hostname: "pbs-test", cfg: cfg}
	c := &pbsClient{cfg: cfg, si: si, http: newHTTPClient(cfg), baseURL: srv.URL + "/api2/json"}

	data, err := c.get("version")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	inner, ok := data["data"].(map[string]any)
	if !ok {
		t.Fatalf("data field: expected map, got %T", data["data"])
	}
	if inner["version"] != "3.1" {
		t.Errorf("version: got %v, want 3.1", inner["version"])
	}
}

func TestPBSGetAuth401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"errors":{"_":"permission check failed"}}`)) //nolint:errcheck
	}))
	defer srv.Close()

	c := newTestPBSClient(srv)
	_, err := c.get("version")
	if err == nil {
		t.Fatal("expected error for 401")
	}
}

func TestPBSGetHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"errors":{"_":"boom"}}`)) //nolint:errcheck
	}))
	defer srv.Close()

	c := newTestPBSClient(srv)
	_, err := c.get("version")
	if err == nil {
		t.Fatal("expected error for non-2xx response")
	}
}

func TestPBSDatastoreUsageParsing(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/status/datastore-usage", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"data": []any{
				map[string]any{
					"store":               "synology",
					"total":               float64(2948636082176),
					"used":                float64(1572864000000),
					"avail":               float64(1375772082176),
					"estimated-full-date": float64(1800000000),
					"mount-status":        "nonremovable",
					"history-start":       float64(1745500000),
					"history-delta":       float64(1800),
					"history":             []any{float64(0.50), float64(0.51), float64(0.52)},
					"gc-status": map[string]any{
						"disk-bytes":       float64(1000000000),
						"disk-chunks":      float64(1000),
						"index-data-bytes": float64(5000000000),
						"index-file-count": float64(100),
						"pending-bytes":    float64(0),
						"pending-chunks":   float64(0),
						"removed-bad":      float64(0),
						"removed-bytes":    float64(0),
						"removed-chunks":   float64(0),
						"still-bad":        float64(0),
						"upid":             "UPID:pbs:0000AA:00001122:000006A2:gc:synology:root@pam:",
					},
				},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestPBSClient(srv)
	data, err := c.get("status/datastore-usage")
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	items, ok := data["data"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected data[]len=1, got %T len=%d", data["data"], len(items))
	}
	ds := items[0].(map[string]any)
	if ds["store"] != "synology" {
		t.Errorf("store: got %v, want synology", ds["store"])
	}
	if ds["mount-status"] != "nonremovable" {
		t.Errorf("mount-status: got %v, want nonremovable", ds["mount-status"])
	}
	gc, ok := ds["gc-status"].(map[string]any)
	if !ok {
		t.Fatalf("gc-status: expected map, got %T", ds["gc-status"])
	}
	if gc["still-bad"] != float64(0) {
		t.Errorf("gc still-bad: got %v, want 0", gc["still-bad"])
	}
}
