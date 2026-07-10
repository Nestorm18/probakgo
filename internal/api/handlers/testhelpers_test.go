package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"probakgo/internal/api"
	dbpkg "probakgo/internal/db"
	"probakgo/internal/service"
	"probakgo/internal/store"
)

type testServer struct {
	handler http.Handler
	store   *store.Store
}

// newTestServer builds a full api.Server backed by an in-memory SQLite DB.
// The returned store can be used to seed API keys and other fixtures.
func newTestServer(t *testing.T) *testServer {
	t.Helper()
	db, err := dbpkg.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.New(db)
	auth := service.NewAuth(st)
	rep := service.NewReport(st, time.UTC)
	srv := api.NewServer(st, auth, rep, nil)
	return &testServer{handler: srv.Router(), store: st}
}

func (ts *testServer) doJSON(t *testing.T, method, path, key string, body any) *httptest.ResponseRecorder {
	return ts.doJSONWithMachine(t, method, path, key, "machine-1", body)
}

func (ts *testServer) doJSONWithMachine(t *testing.T, method, path, key, machineID string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode request body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	if machineID != "" {
		req.Header.Set("X-Machine-ID", machineID)
	}
	rr := httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)
	return rr
}

func decodeJSON(t *testing.T, rr *httptest.ResponseRecorder, out any) {
	t.Helper()
	if err := json.NewDecoder(rr.Body).Decode(out); err != nil {
		t.Fatalf("decode response JSON: %v; body=%s", err, rr.Body.String())
	}
}
