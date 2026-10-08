package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"probakgo/internal/secretbox"
)

type Store struct {
	db      *sql.DB
	secrets *secretbox.Box
	// snapshot is an optional read-only connection for long copies, so a
	// backup does not block the single read-write connection.
	snapshot *sql.DB
}

// SetSnapshotReader sets the read-only connection used by BackupTo.
func (s *Store) SetSnapshotReader(db *sql.DB) {
	s.snapshot = db
}

type dbExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

func NewEncrypted(db *sql.DB, masterKey string) (*Store, error) {
	box, err := secretbox.New(masterKey)
	if err != nil {
		return nil, err
	}
	return &Store{db: db, secrets: box}, nil
}

// sqliteUTC formats t like SQLite's CURRENT_TIMESTAMP. Columns that default to
// it store UTC text, while a bound time.Time is stored as local time with an
// offset, so the two only compare correctly in this format.
func sqliteUTC(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04:05")
}

func (s *Store) DBSize(ctx context.Context) int64 {
	var size int64
	s.db.QueryRowContext(ctx, `SELECT page_count * page_size FROM pragma_page_count(), pragma_page_size()`).Scan(&size)
	return size
}

// Health verifies that SQLite is reachable and the migrated schema can be read.
func (s *Store) Health(ctx context.Context) error {
	if err := s.db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping sqlite: %w", err)
	}
	var migration string
	if err := s.db.QueryRowContext(ctx, `SELECT name FROM schema_migrations ORDER BY name DESC LIMIT 1`).Scan(&migration); err != nil {
		return fmt.Errorf("read schema migrations: %w", err)
	}
	return nil
}

func (s *Store) BackupTo(ctx context.Context, path string) error {
	db := s.db
	if s.snapshot != nil {
		db = s.snapshot
	}
	_, err := db.ExecContext(ctx, `VACUUM INTO '`+strings.ReplaceAll(path, `'`, `''`)+`'`)
	return err
}
