package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendHeartbeatIncludesSwapState(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode heartbeat: %v", err)
			http.Error(w, "invalid heartbeat", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	cfg := &Config{APIURL: server.URL, APIKey: "pbk-test", ServerType: "pve"}
	if err := sendHeartbeat(cfg, &SysInfo{Hostname: "pve-test", cfg: cfg}); err != nil {
		t.Fatalf("sendHeartbeat: %v", err)
	}

	for _, field := range []string{"swap_total", "swap_used", "swap_enabled"} {
		if _, ok := payload[field]; !ok {
			t.Errorf("heartbeat payload does not include %q: %#v", field, payload)
		}
	}
}
