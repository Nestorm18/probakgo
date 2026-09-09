package db

import (
	"context"
	"path/filepath"
	"testing"
)

func TestReopenEnforcesPragmasAndCascade(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.db")
	first, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	var fk int
	var journal string
	if err := first.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatal(err)
	}
	if err := first.QueryRow("PRAGMA journal_mode").Scan(&journal); err != nil {
		t.Fatal(err)
	}
	t.Logf("new database: foreign_keys=%d journal_mode=%s", fk, journal)
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatal(err)
	}
	if err := reopened.QueryRow("PRAGMA journal_mode").Scan(&journal); err != nil {
		t.Fatal(err)
	}
	t.Logf("reopened database: foreign_keys=%d journal_mode=%s", fk, journal)
	if _, err := reopened.Exec("INSERT INTO users (id, username, password_hash, role) VALUES (9000, 'audit', 'hash', 'reader')"); err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.Exec("INSERT INTO push_subscriptions (user_id, endpoint) VALUES (9000, 'https://example.invalid/push')"); err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.Exec("DELETE FROM users WHERE id=9000"); err != nil {
		t.Fatal(err)
	}
	var orphans int
	if err := reopened.QueryRow("SELECT count(*) FROM push_subscriptions WHERE user_id=9000").Scan(&orphans); err != nil {
		t.Fatal(err)
	}
	t.Logf("push subscriptions remaining after user deletion: %d", orphans)
	if fk != 1 || journal != "wal" || orphans != 0 {
		t.Fatalf("foreign_keys=%d journal=%s orphans=%d", fk, journal, orphans)
	}
}

func TestMigrationFailureRollsBackSchemaDataAndMarker(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	for _, script := range []string{
		"CREATE TABLE partial (id INTEGER); INSERT INTO partial VALUES(1); INVALID SQL;",
		"PRAGMA foreign_keys=off; CREATE TABLE partial (id INTEGER); INVALID SQL; PRAGMA foreign_keys=on;",
	} {
		if err := applyMigration(ctx, conn, "regression.sql", script); err == nil {
			t.Fatal("invalid migration succeeded")
		}
		var count, fk int
		if err := conn.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE name='partial'").Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatal("failed migration left a table")
		}
		if err := conn.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations WHERE name='regression.sql'").Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatal("failed migration was marked applied")
		}
		if err := conn.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fk); err != nil {
			t.Fatal(err)
		}
		if fk != 1 {
			t.Fatal("foreign keys not restored after failure")
		}
	}
	if err := applyMigration(ctx, conn, "regression.sql", "CREATE TABLE partial (id INTEGER);"); err != nil {
		t.Fatal(err)
	}
	// Failure to record a migration must also roll its schema changes back.
	if err := applyMigration(ctx, conn, "regression.sql", "CREATE TABLE unrecorded (id INTEGER);"); err == nil {
		t.Fatal("duplicate marker succeeded")
	}
	var count int
	if err := conn.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE name='unrecorded'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("unrecorded migration committed")
	}
}
