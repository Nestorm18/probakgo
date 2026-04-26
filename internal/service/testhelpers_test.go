package service

import (
	"database/sql"
	"testing"

	dbpkg "probakgo/internal/db"
	"probakgo/internal/store"
)

// openTestStore opens an in-memory SQLite DB with all migrations applied and
// returns both the raw *sql.DB (for backdating timestamps in tests) and a *store.Store.
func openTestStore(t *testing.T) (*sql.DB, *store.Store) {
	t.Helper()
	db, err := dbpkg.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db, store.New(db)
}
