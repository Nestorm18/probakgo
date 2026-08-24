package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"probakgo/internal/secretbox"
)

type Store struct {
	db      *sql.DB
	secrets *secretbox.Box
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
	_, err := s.db.ExecContext(ctx, `VACUUM INTO '`+strings.ReplaceAll(path, `'`, `''`)+`'`)
	return err
}
