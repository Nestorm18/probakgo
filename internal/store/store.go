package store

import (
	"context"
	"database/sql"
	"strings"

	"probakgo/internal/secretbox"
)

type Store struct {
	db      *sql.DB
	secrets *secretbox.Box
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

func (s *Store) BackupTo(ctx context.Context, path string) error {
	_, err := s.db.ExecContext(ctx, `VACUUM INTO '`+strings.ReplaceAll(path, `'`, `''`)+`'`)
	return err
}
