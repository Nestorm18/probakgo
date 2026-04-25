package store

import (
	"testing"

	dbpkg "probakgo/internal/db"
)

// openTestDB opens an in-memory SQLite DB with all migrations applied
// and registers a cleanup to close it when the test ends.
func openTestDB(t *testing.T) *Store {
	t.Helper()
	db, err := dbpkg.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return New(db)
}
