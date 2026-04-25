package store

import (
	"database/sql"
	"probaky/internal/domain"
)

func (s *Store) GetUserByUsername(username string) (*domain.User, error) {
	row := s.db.QueryRow(`SELECT id, username, password_hash, role, is_active, created_at
		FROM users WHERE username = ?`, username)
	return scanUser(row)
}

func (s *Store) GetUser(id int64) (*domain.User, error) {
	row := s.db.QueryRow(`SELECT id, username, password_hash, role, is_active, created_at
		FROM users WHERE id = ?`, id)
	return scanUser(row)
}

func (s *Store) ListUsers() ([]domain.User, error) {
	rows, err := s.db.Query(`SELECT id, username, password_hash, role, is_active, created_at
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

func (s *Store) CreateUser(username, hash, role string) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO users (username, password_hash, role) VALUES (?, ?, ?)`, username, hash, role)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateUserPassword(id int64, hash string) error {
	_, err := s.db.Exec(`UPDATE users SET password_hash=? WHERE id=?`, hash, id)
	return err
}

func (s *Store) UpdateUserRole(id int64, role string) error {
	_, err := s.db.Exec(`UPDATE users SET role=? WHERE id=?`, role, id)
	return err
}

func (s *Store) ToggleUser(id int64) error {
	_, err := s.db.Exec(`UPDATE users SET is_active = NOT is_active WHERE id=?`, id)
	return err
}

func (s *Store) DeleteUser(id int64) error {
	_, err := s.db.Exec(`DELETE FROM users WHERE id=?`, id)
	return err
}

func (s *Store) HasUsers() (bool, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
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
