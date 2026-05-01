package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func writeFixture(t *testing.T, payload any) string {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	path := filepath.Join(t.TempDir(), "report.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func TestSendReportFromFilePVE(t *testing.T) {
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/report/pve" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer pbk-testkey" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}
		json.NewDecoder(r.Body).Decode(&received) //nolint:errcheck
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	fixturePath := writeFixture(t, map[string]any{"hostname": "test-node", "ip_address": "192.168.1.1"})

	cfg := &Config{APIURL: srv.URL, APIKey: "pbk-testkey", ServerType: "pve"}
	si := &SysInfo{Hostname: "test-node", cfg: cfg}

	if err := sendReport(cfg, si, fixturePath); err != nil {
		t.Fatalf("sendReport: %v", err)
	}
	if received["hostname"] != "test-node" {
		t.Errorf("hostname: got %v, want test-node", received["hostname"])
	}
}

func TestSendReportFromFilePBS(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/report/pbs" {
			t.Errorf("unexpected path: %s (want /api/report/pbs)", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	fixturePath := writeFixture(t, map[string]any{"hostname": "pbs-node"})

	cfg := &Config{APIURL: srv.URL, APIKey: "pbk-testkey", ServerType: "pbs"}
	si := &SysInfo{Hostname: "pbs-node", cfg: cfg}

	if err := sendReport(cfg, si, fixturePath); err != nil {
		t.Fatalf("sendReport: %v", err)
	}
}

func TestSendReportAuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	fixturePath := writeFixture(t, map[string]any{})
	cfg := &Config{APIURL: srv.URL, APIKey: "pbk-badkey", ServerType: "pve"}
	si := &SysInfo{cfg: cfg}

	err := sendReport(cfg, si, fixturePath)
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
}

func TestSendReportServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	fixturePath := writeFixture(t, map[string]any{})
	cfg := &Config{APIURL: srv.URL, APIKey: "pbk-testkey", ServerType: "pve"}
	si := &SysInfo{cfg: cfg}

	err := sendReport(cfg, si, fixturePath)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestSendReportMissingFile(t *testing.T) {
	cfg := &Config{APIURL: "http://localhost:9", APIKey: "pbk-testkey", ServerType: "pve"}
	si := &SysInfo{cfg: cfg}

	err := sendReport(cfg, si, "/nonexistent/report.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
