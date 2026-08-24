package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthChecksSQLite(t *testing.T) {
	ts := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("healthy status: got %d, want %d: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	if err := ts.db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}
	rr = httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("unhealthy status: got %d, want %d: %s", rr.Code, http.StatusServiceUnavailable, rr.Body.String())
	}
}
