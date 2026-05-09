package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"time"

	"probakgo/internal/debug"
	"probakgo/internal/domain"
)

func (s *Store) GetAPIKeyByValue(ctx context.Context, key string) (*domain.APIKey, error) {
	debug.RecordQuery(ctx, `SELECT id, key, name, key_type, is_active, machine_id, last_used, server_name, server_url, created_at FROM api_keys WHERE key = ?`)
	row := s.db.QueryRowContext(ctx, `SELECT id, key, name, key_type, is_active, machine_id, last_used, server_name, server_url, created_at
		FROM api_keys WHERE key = ?`, key)
	return scanAPIKey(row)
}

func (s *Store) GetAPIKey(ctx context.Context, id int64) (*domain.APIKey, error) {
	debug.RecordQuery(ctx, `SELECT id, key, name, key_type, is_active, machine_id, last_used, server_name, server_url, created_at FROM api_keys WHERE id = ?`)
	row := s.db.QueryRowContext(ctx, `SELECT id, key, name, key_type, is_active, machine_id, last_used, server_name, server_url, created_at
		FROM api_keys WHERE id = ?`, id)
	return scanAPIKey(row)
}

func (s *Store) ListAPIKeys(ctx context.Context) ([]domain.APIKey, error) {
	debug.RecordQuery(ctx, `SELECT id, key, name, key_type, is_active, machine_id, last_used, server_name, server_url, created_at FROM api_keys ORDER BY created_at DESC`)
	rows, err := s.db.QueryContext(ctx, `SELECT id, key, name, key_type, is_active, machine_id, last_used, server_name, server_url, created_at
		FROM api_keys ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []domain.APIKey
	for rows.Next() {
		k, err := scanAPIKeyRow(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, *k)
	}
	return keys, rows.Err()
}

func (s *Store) CreateAPIKey(ctx context.Context, name, serverName, serverURL string) (*domain.APIKey, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	key := fmt.Sprintf("pbk-%x", raw)

	debug.RecordQuery(ctx, `INSERT INTO api_keys (key, name, key_type, server_name, server_url) VALUES (?, ?, 'server', ?, ?)`)
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO api_keys (key, name, key_type, server_name, server_url) VALUES (?, ?, 'server', ?, ?)`,
		key, name, serverName, serverURL,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetAPIKey(ctx, id)
}

func (s *Store) UpdateAPIKeyLastUsed(ctx context.Context, id int64) error {
	debug.RecordQuery(ctx, `UPDATE api_keys SET last_used=? WHERE id=?`)
	_, err := s.db.ExecContext(ctx, `UPDATE api_keys SET last_used=? WHERE id=?`, time.Now(), id)
	return err
}

func (s *Store) BindAPIKeyMachineID(ctx context.Context, id int64, machineID string) error {
	debug.RecordQuery(ctx, `UPDATE api_keys SET machine_id=? WHERE id=?`)
	_, err := s.db.ExecContext(ctx, `UPDATE api_keys SET machine_id=? WHERE id=?`, machineID, id)
	return err
}

func (s *Store) UnbindAPIKeyMachineID(ctx context.Context, id int64) error {
	debug.RecordQuery(ctx, `UPDATE api_keys SET machine_id='' WHERE id=?`)
	_, err := s.db.ExecContext(ctx, `UPDATE api_keys SET machine_id='' WHERE id=?`, id)
	return err
}

func (s *Store) ToggleAPIKey(ctx context.Context, id int64) error {
	debug.RecordQuery(ctx, `UPDATE api_keys SET is_active = NOT is_active WHERE id=?`)
	_, err := s.db.ExecContext(ctx, `UPDATE api_keys SET is_active = NOT is_active WHERE id=?`, id)
	return err
}

func (s *Store) UpdateAPIKey(ctx context.Context, id int64, name, serverName, serverURL string) error {
	debug.RecordQuery(ctx, `UPDATE api_keys SET name=?, server_name=?, server_url=? WHERE id=?`)
	_, err := s.db.ExecContext(ctx, `UPDATE api_keys SET name=?, server_name=?, server_url=? WHERE id=?`, name, serverName, serverURL, id)
	return err
}

func (s *Store) DeleteAPIKey(ctx context.Context, id int64) error {
	debug.RecordQuery(ctx, `DELETE FROM api_keys WHERE id=?`)
	_, err := s.db.ExecContext(ctx, `DELETE FROM api_keys WHERE id=?`, id)
	return err
}

func scanAPIKey(row *sql.Row) (*domain.APIKey, error) {
	var k domain.APIKey
	var isActive int
	var machineID, serverName, serverURL sql.NullString
	var lastUsed sql.NullTime
	err := row.Scan(&k.ID, &k.Key, &k.Name, &k.KeyType, &isActive,
		&machineID, &lastUsed, &serverName, &serverURL, &k.CreatedAt)
	if err != nil {
		return nil, err
	}
	k.IsActive = isActive != 0
	k.MachineID = machineID.String
	k.ServerName = serverName.String
	k.ServerURL = serverURL.String
	if lastUsed.Valid {
		k.LastUsed = &lastUsed.Time
	}
	return &k, nil
}

func scanAPIKeyRow(rows *sql.Rows) (*domain.APIKey, error) {
	var k domain.APIKey
	var isActive int
	var machineID, serverName, serverURL sql.NullString
	var lastUsed sql.NullTime
	err := rows.Scan(&k.ID, &k.Key, &k.Name, &k.KeyType, &isActive,
		&machineID, &lastUsed, &serverName, &serverURL, &k.CreatedAt)
	if err != nil {
		return nil, err
	}
	k.IsActive = isActive != 0
	k.MachineID = machineID.String
	k.ServerName = serverName.String
	k.ServerURL = serverURL.String
	if lastUsed.Valid {
		k.LastUsed = &lastUsed.Time
	}
	return &k, nil
}
