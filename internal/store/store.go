package store

import (
	"context"
	"database/sql"
)

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) DBSize(ctx context.Context) int64 {
	var size int64
	s.db.QueryRowContext(ctx, `SELECT page_count * page_size FROM pragma_page_count(), pragma_page_size()`).Scan(&size)
	return size
}
