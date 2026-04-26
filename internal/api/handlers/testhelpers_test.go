package handlers_test

import (
	"net/http"
	"testing"
	"time"

	dbpkg "probakgo/internal/db"
	"probakgo/internal/api"
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
	srv := api.NewServer(st, auth, rep)
	return &testServer{handler: srv.Router(), store: st}
}
