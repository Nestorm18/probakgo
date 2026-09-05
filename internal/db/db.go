package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"os"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func Open(path string) (*sql.DB, error) {
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	db, err := sql.Open("sqlite", path+separator+"_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	restrictSQLitePermissions(path)
	return db, nil
}

func restrictSQLitePermissions(path string) {
	if path == ":memory:" || strings.HasPrefix(path, "file:") {
		return
	}
	for _, name := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.Chmod(name, 0600); err != nil && !os.IsNotExist(err) {
			slog.Warn("could not restrict sqlite file permissions", "path", name, "err", err)
		}
	}
}

func migrate(db *sql.DB) error {
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		name       TEXT    NOT NULL PRIMARY KEY,
		applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return err
	}

	for _, e := range entries {
		var count int
		if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE name = ?", e.Name()).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			continue
		}

		data, err := migrationsFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			return err
		}
		if err := applyMigration(ctx, conn, e.Name(), string(data)); err != nil {
			return err
		}
		slog.Info("migration applied", "file", e.Name())
	}
	return nil
}

func applyMigration(ctx context.Context, conn *sql.Conn, name, script string) (err error) {
	// Legacy table rebuilds must disable foreign keys outside the transaction.
	// Their embedded PRAGMAs are no-ops inside it; restore enforcement afterwards
	// on this same connection, on both success and rollback.
	if strings.Contains(script, "PRAGMA foreign_keys=off;") {
		if _, err = conn.ExecContext(ctx, "PRAGMA foreign_keys=OFF"); err != nil {
			return err
		}
		defer func() {
			if _, restoreErr := conn.ExecContext(context.Background(), "PRAGMA foreign_keys=ON"); restoreErr != nil && err == nil {
				err = restoreErr
			}
		}()
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, script); err != nil {
		return fmt.Errorf("migration %s: %w", name, err)
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO schema_migrations (name) VALUES (?)", name); err != nil {
		return fmt.Errorf("record migration %s: %w", name, err)
	}
	return tx.Commit()
}
