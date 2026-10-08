package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	dbpkg "probakgo/internal/db"
)

func TestBackupToDoesNotWaitForTheReadWriteConnection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "probakgo_data.db")
	db, err := dbpkg.Open(path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	st := New(db)
	ctx := context.Background()
	if _, err := st.CreateUser(ctx, "admin", "hash", "admin"); err != nil {
		t.Fatalf("create user: %v", err)
	}

	reader, err := dbpkg.OpenSnapshotReader(path)
	if err != nil || reader == nil {
		t.Fatalf("open snapshot reader: %v %v", reader, err)
	}
	t.Cleanup(func() { reader.Close() })
	st.SetSnapshotReader(reader)

	// Hold the only read-write connection, as a long request would.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin write transaction: %v", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO users (username, password_hash, role) VALUES ('in-flight', 'hash', 'reader')`); err != nil {
		t.Fatalf("write in transaction: %v", err)
	}

	backupCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	copyPath := filepath.Join(dir, "copy.db")
	if err := st.BackupTo(backupCtx, copyPath); err != nil {
		t.Fatalf("backup blocked by the write transaction: %v", err)
	}

	copyDB, err := dbpkg.Open(copyPath)
	if err != nil {
		t.Fatalf("open copy: %v", err)
	}
	defer copyDB.Close()
	var users int
	if err := copyDB.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&users); err != nil || users != 1 {
		t.Fatalf("copy should hold the committed user only: users=%d err=%v", users, err)
	}
}

func TestOpenSnapshotReaderSkipsInMemoryDatabases(t *testing.T) {
	for _, path := range []string{":memory:", "file:test?mode=memory"} {
		if reader, err := dbpkg.OpenSnapshotReader(path); reader != nil || err != nil {
			t.Fatalf("%s: got reader=%v err=%v", path, reader, err)
		}
	}
}
