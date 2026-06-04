package db

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMigration018DeletesOrphanAPIKeyServers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "probakgo.db")
	raw, err := sql.Open("sqlite", path+"?_foreign_keys=on&_journal_mode=WAL")
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	if _, err := raw.Exec(`CREATE TABLE schema_migrations (
		name TEXT NOT NULL PRIMARY KEY,
		applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatalf("create schema_migrations: %v", err)
	}
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}
	for _, e := range entries {
		if e.Name() >= "018_delete_orphan_api_key_servers.up.sql" {
			continue
		}
		data, err := migrationsFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			t.Fatalf("read migration %s: %v", e.Name(), err)
		}
		if _, err := raw.Exec(string(data)); err != nil {
			t.Fatalf("apply migration %s: %v", e.Name(), err)
		}
		if _, err := raw.Exec(`INSERT INTO schema_migrations (name) VALUES (?)`, e.Name()); err != nil {
			t.Fatalf("record migration %s: %v", e.Name(), err)
		}
	}

	if _, err := raw.Exec(`INSERT INTO api_keys (id, key, name, key_type, server_name) VALUES (1, 'pbk-valid', 'valid', 'server', 'valid')`); err != nil {
		t.Fatalf("insert api key: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO pve_servers (id, name, api_key_id) VALUES (10, 'orphan', 999), (11, 'valid', 1)`); err != nil {
		t.Fatalf("insert pve servers: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO pve_alert_config (server_id, backup_err) VALUES (10, 1), (11, 1)`); err != nil {
		t.Fatalf("insert alert config: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO alert_suppressions (alert_id, suppressed_until, reason) VALUES ('backup_error:pve:10', 9999999999, 'test')`); err != nil {
		t.Fatalf("insert suppression: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("close raw db: %v", err)
	}

	db, err := Open(path)
	if err != nil {
		t.Fatalf("open migrated db: %v", err)
	}
	defer db.Close()

	assertDBCount(t, db, `SELECT COUNT(*) FROM pve_servers WHERE id = 10`, 0)
	assertDBCount(t, db, `SELECT COUNT(*) FROM pve_alert_config WHERE server_id = 10`, 0)
	assertDBCount(t, db, `SELECT COUNT(*) FROM alert_suppressions WHERE alert_id = 'backup_error:pve:10'`, 0)
	assertDBCount(t, db, `SELECT COUNT(*) FROM pve_servers WHERE id = 11`, 1)
	assertDBCount(t, db, `SELECT COUNT(*) FROM pve_alert_config WHERE server_id = 11`, 1)
}

func TestMigration019DeletesLegacyOrphanServers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "probakgo.db")
	raw, err := sql.Open("sqlite", path+"?_foreign_keys=on&_journal_mode=WAL")
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	if _, err := raw.Exec(`CREATE TABLE schema_migrations (
		name TEXT NOT NULL PRIMARY KEY,
		applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatalf("create schema_migrations: %v", err)
	}
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}
	for _, e := range entries {
		if e.Name() >= "019_delete_legacy_orphan_servers.up.sql" {
			continue
		}
		data, err := migrationsFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			t.Fatalf("read migration %s: %v", e.Name(), err)
		}
		if _, err := raw.Exec(string(data)); err != nil {
			t.Fatalf("apply migration %s: %v", e.Name(), err)
		}
		if _, err := raw.Exec(`INSERT INTO schema_migrations (name) VALUES (?)`, e.Name()); err != nil {
			t.Fatalf("record migration %s: %v", e.Name(), err)
		}
	}

	if _, err := raw.Exec(`INSERT INTO api_keys (id, key, name, key_type, server_name) VALUES (1, 'pbk-valid', 'valid', 'server', 'valid')`); err != nil {
		t.Fatalf("insert api key: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO pve_servers (id, name, api_key_id) VALUES (20, 'legacy-orphan', NULL), (21, 'valid', NULL)`); err != nil {
		t.Fatalf("insert pve servers: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO pve_alert_config (server_id, backup_err) VALUES (20, 1), (21, 1)`); err != nil {
		t.Fatalf("insert alert config: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO alert_suppressions (alert_id, suppressed_until, reason) VALUES ('backup_error:pve:20', 9999999999, 'test')`); err != nil {
		t.Fatalf("insert suppression: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("close raw db: %v", err)
	}

	db, err := Open(path)
	if err != nil {
		t.Fatalf("open migrated db: %v", err)
	}
	defer db.Close()

	assertDBCount(t, db, `SELECT COUNT(*) FROM pve_servers WHERE id = 20`, 0)
	assertDBCount(t, db, `SELECT COUNT(*) FROM pve_alert_config WHERE server_id = 20`, 0)
	assertDBCount(t, db, `SELECT COUNT(*) FROM alert_suppressions WHERE alert_id = 'backup_error:pve:20'`, 0)
	assertDBCount(t, db, `SELECT COUNT(*) FROM pve_servers WHERE id = 21`, 1)
	assertDBCount(t, db, `SELECT COUNT(*) FROM pve_alert_config WHERE server_id = 21`, 1)
}

func assertDBCount(t *testing.T, db *sql.DB, query string, want int) {
	t.Helper()
	var got int
	if err := db.QueryRow(query).Scan(&got); err != nil {
		t.Fatalf("count query %q: %v", query, err)
	}
	if got != want {
		t.Fatalf("count query %q got %d, want %d", query, got, want)
	}
}
