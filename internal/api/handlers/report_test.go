package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReportPVE_HappyPath(t *testing.T) {
	ctx := context.Background()
	ts := newTestServer(t)
	k, err := ts.store.CreateAPIKey(ctx, "client", "", "")
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}

	body := `{"hostname":"pve-01","ip_address":"10.0.0.1","storages":[]}`
	req := httptest.NewRequest(http.MethodPost, "/report/pve", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+k.Key)
	req.Header.Set("X-Machine-ID", "machine-1")

	rr := httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestReportPVE_MissingHostname(t *testing.T) {
	ctx := context.Background()
	ts := newTestServer(t)
	k, _ := ts.store.CreateAPIKey(ctx, "client", "", "")

	body := `{"hostname":"","ip_address":"10.0.0.1"}`
	req := httptest.NewRequest(http.MethodPost, "/report/pve", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+k.Key)
	req.Header.Set("X-Machine-ID", "machine-1")

	rr := httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestReportPVE_InvalidJSON(t *testing.T) {
	ctx := context.Background()
	ts := newTestServer(t)
	k, _ := ts.store.CreateAPIKey(ctx, "client", "", "")

	req := httptest.NewRequest(http.MethodPost, "/report/pve", strings.NewReader("{bad json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+k.Key)
	req.Header.Set("X-Machine-ID", "machine-1")

	rr := httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestReportPVE_MissingMachineID(t *testing.T) {
	ctx := context.Background()
	ts := newTestServer(t)
	k, _ := ts.store.CreateAPIKey(ctx, "client", "", "")

	body := `{"hostname":"pve-01","ip_address":"10.0.0.1","storages":[]}`
	req := httptest.NewRequest(http.MethodPost, "/report/pve", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+k.Key)

	rr := httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestReportPVE_BoundServerNameMismatchRejected(t *testing.T) {
	ctx := context.Background()
	ts := newTestServer(t)
	k, _ := ts.store.CreateAPIKey(ctx, "client", "pve-01", "")

	body := `{"hostname":"pve-02","ip_address":"10.0.0.2","storages":[]}`
	req := httptest.NewRequest(http.MethodPost, "/report/pve", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+k.Key)
	req.Header.Set("X-Machine-ID", "machine-1")

	rr := httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `expected \"pve-01\", got \"pve-02\"`) {
		t.Fatalf("response should explain server mismatch, got: %s", rr.Body.String())
	}
}

func TestReportPVE_ServerNameTrimmed(t *testing.T) {
	ctx := context.Background()
	ts := newTestServer(t)
	k, _ := ts.store.CreateAPIKey(ctx, "client", "nicolas-gestion ", "")

	body := `{"hostname":"nicolas-gestion","ip_address":"10.0.0.1","storages":[]}`
	req := httptest.NewRequest(http.MethodPost, "/report/pve", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+k.Key)
	req.Header.Set("X-Machine-ID", "machine-1")

	rr := httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestReportPVE_MachineIDMismatch(t *testing.T) {
	ctx := context.Background()
	ts := newTestServer(t)
	k, _ := ts.store.CreateAPIKey(ctx, "client", "", "")

	body := `{"hostname":"pve-01","ip_address":"10.0.0.1","storages":[]}`
	req := httptest.NewRequest(http.MethodPost, "/report/pve", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+k.Key)
	req.Header.Set("X-Machine-ID", "machine-1")

	rr := httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("first report: want 200, got %d: %s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/report/pve", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+k.Key)
	req.Header.Set("X-Machine-ID", "machine-2")

	rr = httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("second report: want 401, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestReportPVE_UnbindAllowsDifferentServer(t *testing.T) {
	ctx := context.Background()
	ts := newTestServer(t)
	k, _ := ts.store.CreateAPIKey(ctx, "client", "", "")

	body := `{"hostname":"old-pve","ip_address":"10.0.0.1","storages":[]}`
	req := httptest.NewRequest(http.MethodPost, "/report/pve", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+k.Key)
	req.Header.Set("X-Machine-ID", "machine-old")

	rr := httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("first report: want 200, got %d: %s", rr.Code, rr.Body.String())
	}

	if err := ts.store.UnbindAPIKeyServer(ctx, k.ID); err != nil {
		t.Fatalf("unbind api key: %v", err)
	}

	body = `{"hostname":"nicolas","ip_address":"10.0.0.2","storages":[]}`
	req = httptest.NewRequest(http.MethodPost, "/report/pve", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+k.Key)
	req.Header.Set("X-Machine-ID", "machine-new")

	rr = httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("after unbind: want 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestReportPVE_DuplicateHostnameSeparatedByAPIKey(t *testing.T) {
	ctx := context.Background()
	ts := newTestServer(t)
	keyA, _ := ts.store.CreateAPIKey(ctx, "pbs-a", "same-host", "")
	keyB, _ := ts.store.CreateAPIKey(ctx, "pbs-b", "same-host", "")

	body := `{"hostname":"same-host","ip_address":"10.0.0.1","storages":[]}`
	req := httptest.NewRequest(http.MethodPost, "/report/pve", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+keyA.Key)
	req.Header.Set("X-Machine-ID", "machine-a")
	rr := httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("first report: want 200, got %d: %s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/report/pve", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+keyB.Key)
	req.Header.Set("X-Machine-ID", "machine-b")
	rr = httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("second report: want 200, got %d: %s", rr.Code, rr.Body.String())
	}

	servers, err := ts.store.ListPVEServers(ctx)
	if err != nil {
		t.Fatalf("ListPVEServers: %v", err)
	}
	if len(servers) != 2 {
		t.Fatalf("servers: got %d, want 2", len(servers))
	}
	if servers[0].ID == servers[1].ID || servers[0].APIKeyID == servers[1].APIKeyID {
		t.Fatalf("servers were not separated by API key: %+v", servers)
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
	ctx := context.Background()
	ts := newTestServer(t)
	k, err := ts.store.CreateAPIKey(ctx, "client", "", "")
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}

	body := `{"hostname":"pbs-01","ip_address":"10.0.0.2","pbs_information":{"data":[]}}`
	req := httptest.NewRequest(http.MethodPost, "/report/pbs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+k.Key)
	req.Header.Set("X-Machine-ID", "machine-1")

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
