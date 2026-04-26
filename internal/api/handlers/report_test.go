package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReportPVE_HappyPath(t *testing.T) {
	ts := newTestServer(t)
	k, err := ts.store.CreateAPIKey("client", "server", "")
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}

	body := `{"hostname":"pve-01","ip_address":"10.0.0.1","storages":[]}`
	req := httptest.NewRequest(http.MethodPost, "/report/pve", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+k.Key)

	rr := httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestReportPVE_MissingHostname(t *testing.T) {
	ts := newTestServer(t)
	k, _ := ts.store.CreateAPIKey("client", "server", "")

	body := `{"hostname":"","ip_address":"10.0.0.1"}`
	req := httptest.NewRequest(http.MethodPost, "/report/pve", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+k.Key)

	rr := httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestReportPVE_InvalidJSON(t *testing.T) {
	ts := newTestServer(t)
	k, _ := ts.store.CreateAPIKey("client", "server", "")

	req := httptest.NewRequest(http.MethodPost, "/report/pve", strings.NewReader("{bad json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+k.Key)

	rr := httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestReportPVE_NoAuth(t *testing.T) {
	ts := newTestServer(t)

	body := `{"hostname":"pve-01","ip_address":"10.0.0.1"}`
	req := httptest.NewRequest(http.MethodPost, "/report/pve", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestReportPBS_HappyPath(t *testing.T) {
	ts := newTestServer(t)
	k, err := ts.store.CreateAPIKey("client", "server", "")
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}

	body := `{"hostname":"pbs-01","ip_address":"10.0.0.2","pbs_information":{"data":[]}}`
	req := httptest.NewRequest(http.MethodPost, "/report/pbs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+k.Key)

	rr := httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestReportPBS_NoAuth(t *testing.T) {
	ts := newTestServer(t)

	body := `{"hostname":"pbs-01","ip_address":"10.0.0.2","pbs_information":{"data":[]}}`
	req := httptest.NewRequest(http.MethodPost, "/report/pbs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d: %s", rr.Code, rr.Body.String())
	}
}
