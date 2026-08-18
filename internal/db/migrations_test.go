package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	_ "modernc.org/sqlite"
)

func TestOpenRestrictsSQLitePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix file permissions are not available on Windows")
	}

	path := filepath.Join(t.TempDir(), "probakgo.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat database: %v", err)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0600); got != want {
		t.Fatalf("database permissions = %04o, want %04o", got, want)
	}
}

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

func TestMigration020MergesServerIdentityAndDropsOldVMUnique(t *testing.T) {
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
		if e.Name() >= "020_merge_server_identity_and_vm_config_unique.up.sql" {
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

	if _, err := raw.Exec(`INSERT INTO api_keys (id, key, name, key_type, server_name, machine_id) VALUES (1, 'pbk-merge', 'alias1', 'server', 'host1', 'machine-1')`); err != nil {
		t.Fatalf("insert api key: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO pve_servers (id, name, machine_id, api_key_id) VALUES (30, 'host1', 'machine-1', NULL), (31, 'host1', 'machine-1', 1)`); err != nil {
		t.Fatalf("insert pve servers: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO pve_reports (id, server_id, backup_status) VALUES (300, 30, 'OK'), (301, 31, 'OK')`); err != nil {
		t.Fatalf("insert reports: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO server_heartbeats (server_type, server_id, hostname, last_seen_at) VALUES ('pve', 31, 'host1', CURRENT_TIMESTAMP)`); err != nil {
		t.Fatalf("insert heartbeat: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("close raw db: %v", err)
	}

	db, err := Open(path)
	if err != nil {
		t.Fatalf("open migrated db: %v", err)
	}
	defer db.Close()

	assertDBCount(t, db, `SELECT COUNT(*) FROM pve_servers WHERE id = 30 AND api_key_id = 1`, 1)
	assertDBCount(t, db, `SELECT COUNT(*) FROM pve_servers WHERE id = 31`, 0)
	assertDBCount(t, db, `SELECT COUNT(*) FROM pve_reports WHERE server_id = 30`, 2)
	assertDBCount(t, db, `SELECT COUNT(*) FROM server_heartbeats WHERE server_type = 'pve' AND server_id = 30`, 1)
	if _, err := db.Exec(`INSERT INTO vm_backup_configs (server_name, server_type, server_id, vm_id) VALUES ('same-host', 'pve', 100, '101'), ('same-host', 'pve', 101, '101')`); err != nil {
		t.Fatalf("same hostname vm configs should be allowed for different server IDs: %v", err)
	}
}

func TestMigration042SetsGlobalPVEFinishTimeBeforeEmail(t *testing.T) {
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
		if e.Name() >= "042_global_pve_expected_finish_time.up.sql" {
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
	if _, err := raw.Exec(`INSERT INTO email_config (send_time) VALUES ('08:30')`); err != nil {
		t.Fatalf("insert email config: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("close raw db: %v", err)
	}

	database, err := Open(path)
	if err != nil {
		t.Fatalf("open migrated db: %v", err)
	}
	defer database.Close()

	var got string
	if err := database.QueryRow(`SELECT alert_pve_expected_finish_time FROM email_config`).Scan(&got); err != nil {
		t.Fatalf("read global PVE finish time: %v", err)
	}
	if got != "08:25" {
		t.Fatalf("global PVE finish time = %q, want five minutes before email at 08:30", got)
	}
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

func TestMigration044RepairsPreviouslyRecordedTelegramMigration(t *testing.T) {
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
	for _, entry := range entries {
		if entry.Name() >= "043_telegram_notifications.up.sql" {
			continue
		}
		data, err := migrationsFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			t.Fatalf("read migration %s: %v", entry.Name(), err)
		}
		if _, err := raw.Exec(string(data)); err != nil {
			t.Fatalf("apply migration %s: %v", entry.Name(), err)
		}
		if _, err := raw.Exec(`INSERT INTO schema_migrations (name) VALUES (?)`, entry.Name()); err != nil {
			t.Fatalf("record migration %s: %v", entry.Name(), err)
		}
	}
	if _, err := raw.Exec(`
		CREATE TABLE telegram_config (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			bot_token TEXT NOT NULL DEFAULT '',
			bot_username TEXT NOT NULL DEFAULT '',
			chat_id TEXT NOT NULL DEFAULT '',
			chat_title TEXT NOT NULL DEFAULT '',
			is_enabled INTEGER NOT NULL DEFAULT 0,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE telegram_delivery_status (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			last_attempt_at DATETIME,
			last_success_at DATETIME,
			last_error TEXT NOT NULL DEFAULT ''
		);
		INSERT INTO telegram_config (id, bot_token, bot_username, chat_id, chat_title, is_enabled)
		VALUES (1, 'legacy-token', 'legacy_bot', '123456789', 'Nestor', 1);
		INSERT INTO schema_migrations (name) VALUES ('043_telegram_notifications.up.sql');
	`); err != nil {
		t.Fatalf("create legacy Telegram schema: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("close raw db: %v", err)
	}

	database, err := Open(path)
	if err != nil {
		t.Fatalf("open repaired database: %v", err)
	}
	defer database.Close()

	assertDBCount(t, database, `SELECT COUNT(*) FROM schema_migrations WHERE name = '044_telegram_user_links.up.sql'`, 1)
	assertDBCount(t, database, `SELECT COUNT(*) FROM telegram_destinations`, 0)
	var token, username string
	if err := database.QueryRow(`SELECT bot_token, bot_username FROM telegram_config WHERE id = 1`).Scan(&token, &username); err != nil {
		t.Fatalf("read preserved Telegram bot: %v", err)
	}
	if token != "legacy-token" || username != "legacy_bot" {
		t.Fatalf("Telegram bot config changed: token=%q username=%q", token, username)
	}
	if _, err := database.Exec(`INSERT INTO users (username, password_hash, role) VALUES ('alice', 'hash', 'reader')`); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := database.Exec(`INSERT INTO telegram_destinations (user_id, chat_id, chat_title) SELECT id, '987654321', 'Alice' FROM users WHERE username = 'alice'`); err != nil {
		t.Fatalf("insert repaired Telegram link: %v", err)
	}
	assertDBCount(t, database, `SELECT COUNT(*) FROM telegram_destinations`, 1)
}
