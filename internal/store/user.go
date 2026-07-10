package store

import (
	"context"
	"database/sql"

	"probakgo/internal/debug"
	"probakgo/internal/domain"
)

func (s *Store) GetUserByUsername(ctx context.Context, username string) (*domain.User, error) {
	debug.RecordQuery(ctx, `SELECT id, username, password_hash, role, is_active, created_at, last_login_at, last_login_ip, totp_enabled, totp_secret, totp_confirmed_at, totp_grace_started_at, session_version FROM users WHERE username = ?`)
	row := s.db.QueryRowContext(ctx, `SELECT id, username, password_hash, role, is_active, created_at, last_login_at, last_login_ip,
		totp_enabled, totp_secret, totp_confirmed_at, totp_grace_started_at, session_version
		FROM users WHERE username = ?`, username)
	return scanUser(row)
}

func (s *Store) GetUser(ctx context.Context, id int64) (*domain.User, error) {
	debug.RecordQuery(ctx, `SELECT id, username, password_hash, role, is_active, created_at, last_login_at, last_login_ip, totp_enabled, totp_secret, totp_confirmed_at, totp_grace_started_at, session_version FROM users WHERE id = ?`)
	row := s.db.QueryRowContext(ctx, `SELECT id, username, password_hash, role, is_active, created_at, last_login_at, last_login_ip,
		totp_enabled, totp_secret, totp_confirmed_at, totp_grace_started_at, session_version
		FROM users WHERE id = ?`, id)
	return scanUser(row)
}

func (s *Store) ListUsers(ctx context.Context) ([]domain.User, error) {
	debug.RecordQuery(ctx, `SELECT id, username, password_hash, role, is_active, created_at, last_login_at, last_login_ip, totp_enabled, totp_secret, totp_confirmed_at, totp_grace_started_at, session_version FROM users ORDER BY username`)
	rows, err := s.db.QueryContext(ctx, `SELECT id, username, password_hash, role, is_active, created_at, last_login_at, last_login_ip,
		totp_enabled, totp_secret, totp_confirmed_at, totp_grace_started_at, session_version
		FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []domain.User
	for rows.Next() {
		var u domain.User
		var isActive int
		var lastLoginAt sql.NullTime
		var lastLoginIP sql.NullString
		var totpEnabled int
		var totpSecret sql.NullString
		var totpConfirmedAt sql.NullTime
		var totpGraceStartedAt sql.NullTime
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &isActive, &u.CreatedAt, &lastLoginAt, &lastLoginIP,
			&totpEnabled, &totpSecret, &totpConfirmedAt, &totpGraceStartedAt, &u.SessionVersion); err != nil {
			return nil, err
		}
		u.IsActive = isActive != 0
		if lastLoginAt.Valid {
			u.LastLoginAt = &lastLoginAt.Time
		}
		u.LastLoginIP = lastLoginIP.String
		u.TOTPEnabled = totpEnabled != 0
		u.TOTPSecret = totpSecret.String
		if totpConfirmedAt.Valid {
			u.TOTPConfirmedAt = &totpConfirmedAt.Time
		}
		if totpGraceStartedAt.Valid {
			u.TOTPGraceStartedAt = &totpGraceStartedAt.Time
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *Store) UpdateUserLastLogin(ctx context.Context, id int64, ip string) error {
	debug.RecordQuery(ctx, `UPDATE users SET last_login_at = CURRENT_TIMESTAMP, last_login_ip = ? WHERE id = ?`)
	_, err := s.db.ExecContext(ctx, `UPDATE users SET last_login_at = CURRENT_TIMESTAMP, last_login_ip = ? WHERE id = ?`, ip, id)
	return err
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
	debug.RecordQuery(ctx, `UPDATE users SET password_hash=?, session_version=session_version+1 WHERE id=?`)
	_, err := s.db.ExecContext(ctx, `UPDATE users SET password_hash=?, session_version=session_version+1 WHERE id=?`, hash, id)
	return err
}

func (s *Store) EnableUserTOTP(ctx context.Context, id int64, secret string) error {
	debug.RecordQuery(ctx, `UPDATE users SET totp_enabled=1, totp_secret=?, totp_confirmed_at=CURRENT_TIMESTAMP, totp_grace_started_at=NULL, session_version=session_version+1 WHERE id=?`)
	_, err := s.db.ExecContext(ctx, `UPDATE users SET totp_enabled=1, totp_secret=?, totp_confirmed_at=CURRENT_TIMESTAMP, totp_grace_started_at=NULL, session_version=session_version+1 WHERE id=?`, secret, id)
	return err
}

func (s *Store) DisableUserTOTP(ctx context.Context, id int64) error {
	debug.RecordQuery(ctx, `UPDATE users SET totp_enabled=0, totp_secret='', totp_confirmed_at=NULL, totp_grace_started_at=NULL, session_version=session_version+1 WHERE id=?`)
	_, err := s.db.ExecContext(ctx, `UPDATE users SET totp_enabled=0, totp_secret='', totp_confirmed_at=NULL, totp_grace_started_at=NULL, session_version=session_version+1 WHERE id=?`, id)
	return err
}

func (s *Store) DisableUserTOTPByUsername(ctx context.Context, username string) (bool, error) {
	debug.RecordQuery(ctx, `UPDATE users SET totp_enabled=0, totp_secret='', totp_confirmed_at=NULL, totp_grace_started_at=NULL, session_version=session_version+1 WHERE username=?`)
	res, err := s.db.ExecContext(ctx, `UPDATE users SET totp_enabled=0, totp_secret='', totp_confirmed_at=NULL, totp_grace_started_at=NULL, session_version=session_version+1 WHERE username=?`, username)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (s *Store) StartUserTOTPGrace(ctx context.Context, id int64) error {
	debug.RecordQuery(ctx, `UPDATE users SET totp_grace_started_at = COALESCE(totp_grace_started_at, CURRENT_TIMESTAMP) WHERE id=?`)
	_, err := s.db.ExecContext(ctx, `UPDATE users SET totp_grace_started_at = COALESCE(totp_grace_started_at, CURRENT_TIMESTAMP) WHERE id=?`, id)
	return err
}

func (s *Store) ClearUserTOTPGrace(ctx context.Context) error {
	debug.RecordQuery(ctx, `UPDATE users SET totp_grace_started_at=NULL`)
	_, err := s.db.ExecContext(ctx, `UPDATE users SET totp_grace_started_at=NULL`)
	return err
}

func (s *Store) SetUserActive(ctx context.Context, id int64, active bool) error {
	debug.RecordQuery(ctx, `UPDATE users SET is_active=?, session_version=session_version+1 WHERE id=?`)
	_, err := s.db.ExecContext(ctx, `UPDATE users SET is_active=?, session_version=session_version+1 WHERE id=?`, boolToInt(active), id)
	return err
}

func (s *Store) UpdateUserRole(ctx context.Context, id int64, role string) error {
	debug.RecordQuery(ctx, `UPDATE users SET role=?, session_version=session_version+1 WHERE id=?`)
	_, err := s.db.ExecContext(ctx, `UPDATE users SET role=?, session_version=session_version+1 WHERE id=?`, role, id)
	return err
}

func (s *Store) ToggleUser(ctx context.Context, id int64) error {
	debug.RecordQuery(ctx, `UPDATE users SET is_active = NOT is_active, session_version=session_version+1 WHERE id=?`)
	_, err := s.db.ExecContext(ctx, `UPDATE users SET is_active = NOT is_active, session_version=session_version+1 WHERE id=?`, id)
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
	var lastLoginAt sql.NullTime
	var lastLoginIP sql.NullString
	var totpEnabled int
	var totpSecret sql.NullString
	var totpConfirmedAt sql.NullTime
	var totpGraceStartedAt sql.NullTime
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &isActive, &u.CreatedAt, &lastLoginAt, &lastLoginIP,
		&totpEnabled, &totpSecret, &totpConfirmedAt, &totpGraceStartedAt, &u.SessionVersion); err != nil {
		return nil, err
	}
	u.IsActive = isActive != 0
	if lastLoginAt.Valid {
		u.LastLoginAt = &lastLoginAt.Time
	}
	u.LastLoginIP = lastLoginIP.String
	u.TOTPEnabled = totpEnabled != 0
	u.TOTPSecret = totpSecret.String
	if totpConfirmedAt.Valid {
		u.TOTPConfirmedAt = &totpConfirmedAt.Time
	}
	if totpGraceStartedAt.Valid {
		u.TOTPGraceStartedAt = &totpGraceStartedAt.Time
	}
	return &u, nil
}
