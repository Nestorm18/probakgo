package store

import (
	"context"
	"database/sql"

	"probakgo/internal/debug"
	"probakgo/internal/domain"
)

func (s *Store) GetUserByUsername(ctx context.Context, username string) (*domain.User, error) {
	debug.RecordQuery(ctx, `SELECT id, username, password_hash, role, is_active, created_at FROM users WHERE username = ?`)
	row := s.db.QueryRowContext(ctx, `SELECT id, username, password_hash, role, is_active, created_at
		FROM users WHERE username = ?`, username)
	return scanUser(row)
}

func (s *Store) GetUser(ctx context.Context, id int64) (*domain.User, error) {
	debug.RecordQuery(ctx, `SELECT id, username, password_hash, role, is_active, created_at FROM users WHERE id = ?`)
	row := s.db.QueryRowContext(ctx, `SELECT id, username, password_hash, role, is_active, created_at
		FROM users WHERE id = ?`, id)
	return scanUser(row)
}

func (s *Store) ListUsers(ctx context.Context) ([]domain.User, error) {
	debug.RecordQuery(ctx, `SELECT id, username, password_hash, role, is_active, created_at FROM users ORDER BY username`)
	rows, err := s.db.QueryContext(ctx, `SELECT id, username, password_hash, role, is_active, created_at
		FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []domain.User
	for rows.Next() {
		var u domain.User
		var isActive int
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &isActive, &u.CreatedAt); err != nil {
			return nil, err
		}
		u.IsActive = isActive != 0
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *Store) CreateUser(ctx context.Context, username, hash, role string) (int64, error) {
	debug.RecordQuery(ctx, `INSERT INTO users (username, password_hash, role) VALUES (?, ?, ?)`)
	res, err := s.db.ExecContext(ctx, `INSERT INTO users (username, password_hash, role) VALUES (?, ?, ?)`, username, hash, role)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateUserUsername(ctx context.Context, id int64, username string) error {
	debug.RecordQuery(ctx, `UPDATE users SET username=? WHERE id=?`)
	_, err := s.db.ExecContext(ctx, `UPDATE users SET username=? WHERE id=?`, username, id)
	return err
}

func (s *Store) UpdateUserPassword(ctx context.Context, id int64, hash string) error {
	debug.RecordQuery(ctx, `UPDATE users SET password_hash=? WHERE id=?`)
	_, err := s.db.ExecContext(ctx, `UPDATE users SET password_hash=? WHERE id=?`, hash, id)
	return err
}

func (s *Store) UpdateUserRole(ctx context.Context, id int64, role string) error {
	debug.RecordQuery(ctx, `UPDATE users SET role=? WHERE id=?`)
	_, err := s.db.ExecContext(ctx, `UPDATE users SET role=? WHERE id=?`, role, id)
	return err
}

func (s *Store) ToggleUser(ctx context.Context, id int64) error {
	debug.RecordQuery(ctx, `UPDATE users SET is_active = NOT is_active WHERE id=?`)
	_, err := s.db.ExecContext(ctx, `UPDATE users SET is_active = NOT is_active WHERE id=?`, id)
	return err
}

func (s *Store) DeleteUser(ctx context.Context, id int64) error {
	debug.RecordQuery(ctx, `DELETE FROM users WHERE id=?`)
	_, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id=?`, id)
	return err
}

func (s *Store) HasUsers(ctx context.Context) (bool, error) {
	debug.RecordQuery(ctx, `SELECT COUNT(*) FROM users`)
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	return count > 0, err
}

func scanUser(row *sql.Row) (*domain.User, error) {
	var u domain.User
	var isActive int
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &isActive, &u.CreatedAt); err != nil {
		return nil, err
	}
	u.IsActive = isActive != 0
	return &u, nil
}
