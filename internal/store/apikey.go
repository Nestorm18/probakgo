package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"probakgo/internal/debug"
	"probakgo/internal/domain"
)

func (s *Store) GetAPIKeyByValue(ctx context.Context, key string) (*domain.APIKey, error) {
	if s.secrets != nil {
		debug.RecordQuery(ctx, `SELECT ... FROM api_keys WHERE key_hash = ?`)
		row := s.db.QueryRowContext(ctx, `SELECT id, key, name, key_type, is_active, machine_id, last_used, server_name, server_url, created_at
			FROM api_keys WHERE key_hash = ?`, s.secrets.LookupHash(key))
		k, err := s.scanAPIKey(row)
		if err == nil || err != sql.ErrNoRows {
			return k, err
		}
	}

	debug.RecordQuery(ctx, `SELECT id, key, name, key_type, is_active, machine_id, last_used, server_name, server_url, created_at FROM api_keys WHERE key = ?`)
	row := s.db.QueryRowContext(ctx, `SELECT id, key, name, key_type, is_active, machine_id, last_used, server_name, server_url, created_at
		FROM api_keys WHERE key = ?`, key)
	k, err := s.scanAPIKey(row)
	if err != nil || s.secrets == nil {
		return k, err
	}
	if err := s.protectAPIKeyValue(ctx, k.ID, key); err != nil {
		return nil, err
	}
	return k, nil
}

func (s *Store) GetAPIKey(ctx context.Context, id int64) (*domain.APIKey, error) {
	debug.RecordQuery(ctx, `SELECT id, key, name, key_type, is_active, machine_id, last_used, server_name, server_url, created_at FROM api_keys WHERE id = ?`)
	row := s.db.QueryRowContext(ctx, `SELECT id, key, name, key_type, is_active, machine_id, last_used, server_name, server_url, created_at
		FROM api_keys WHERE id = ?`, id)
	return s.scanAPIKey(row)
}

func (s *Store) ListAPIKeys(ctx context.Context) ([]domain.APIKey, error) {
	return s.ListAPIKeysPage(ctx, 0, 0, "")
}

func (s *Store) ListAPIKeysPage(ctx context.Context, limit, offset int, query string) ([]domain.APIKey, error) {
	if offset < 0 {
		offset = 0
	}
	query = strings.TrimSpace(query)
	where := ""
	args := []any{}
	if query != "" {
		like := "%" + strings.ToLower(query) + "%"
		where = ` WHERE lower(name) LIKE ? OR lower(server_name) LIKE ? OR lower(machine_id) LIKE ? OR lower(server_url) LIKE ? OR lower(key) LIKE ?`
		args = append(args, like, like, like, like, like)
		if s.secrets != nil && strings.HasPrefix(query, "pbk-") {
			where += ` OR key_hash = ?`
			args = append(args, s.secrets.LookupHash(query))
		}
		switch strings.ToLower(query) {
		case "activa", "activo", "active":
			where += ` OR is_active = 1`
		case "inactiva", "inactivo", "inactive":
			where += ` OR is_active = 0`
		}
	}
	limitSQL := ""
	if limit > 0 {
		limitSQL = ` LIMIT ? OFFSET ?`
		args = append(args, limit, offset)
	}

	debug.RecordQuery(ctx, `SELECT id, key, name, key_type, is_active, machine_id, last_used, server_name, server_url, created_at FROM api_keys ORDER BY created_at DESC`)
	rows, err := s.db.QueryContext(ctx, `SELECT id, key, name, key_type, is_active, machine_id, last_used, server_name, server_url, created_at
		FROM api_keys`+where+` ORDER BY created_at DESC, id DESC`+limitSQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []domain.APIKey
	for rows.Next() {
		k, err := s.scanAPIKeyRow(rows)
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
	name = strings.TrimSpace(name)
	serverName = strings.TrimSpace(serverName)
	serverURL = strings.TrimSpace(serverURL)
	if name == "" {
		name = serverName
	}
	storedKey := key
	keyHash := ""
	if s.secrets != nil {
		var err error
		storedKey, err = s.secrets.Encrypt(key)
		if err != nil {
			return nil, err
		}
		keyHash = s.secrets.LookupHash(key)
	}

	debug.RecordQuery(ctx, `INSERT INTO api_keys (key, key_hash, name, key_type, server_name, server_url) VALUES (?, ?, ?, 'server', ?, ?)`)
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO api_keys (key, key_hash, name, key_type, server_name, server_url) VALUES (?, ?, ?, 'server', ?, ?)`,
		storedKey, keyHash, name, serverName, serverURL,
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

func (s *Store) BindAPIKeyServerName(ctx context.Context, id int64, serverName string) error {
	debug.RecordQuery(ctx, `UPDATE api_keys SET server_name=? WHERE id=?`)
	_, err := s.db.ExecContext(ctx,
		`UPDATE api_keys SET server_name=? WHERE id=?`,
		serverName, id,
	)
	return err
}

func (s *Store) UnbindAPIKeyServer(ctx context.Context, id int64) error {
	debug.RecordQuery(ctx, `UPDATE api_keys SET machine_id='', server_name='' WHERE id=?`)
	_, err := s.db.ExecContext(ctx, `UPDATE api_keys SET machine_id='', server_name='' WHERE id=?`, id)
	return err
}

func (s *Store) ToggleAPIKey(ctx context.Context, id int64) error {
	debug.RecordQuery(ctx, `UPDATE api_keys SET is_active = NOT is_active WHERE id=?`)
	_, err := s.db.ExecContext(ctx, `UPDATE api_keys SET is_active = NOT is_active WHERE id=?`, id)
	return err
}

func (s *Store) UpdateAPIKey(ctx context.Context, id int64, name, serverName, serverURL string) error {
	name = strings.TrimSpace(name)
	serverName = strings.TrimSpace(serverName)
	serverURL = strings.TrimSpace(serverURL)
	if name == "" {
		name = serverName
	}
	debug.RecordQuery(ctx, `UPDATE api_keys SET name=?, server_name=?, server_url=? WHERE id=?`)
	_, err := s.db.ExecContext(ctx, `UPDATE api_keys SET name=?, server_name=?, server_url=? WHERE id=?`, name, serverName, serverURL, id)
	return err
}

func (s *Store) DeleteAPIKey(ctx context.Context, id int64) error {
	debug.RecordQuery(ctx, `DELETE FROM api_keys WHERE id=?`)
	_, err := s.db.ExecContext(ctx, `DELETE FROM api_keys WHERE id=?`, id)
	return err
}

func (s *Store) scanAPIKey(row *sql.Row) (*domain.APIKey, error) {
	var k domain.APIKey
	var isActive int
	var machineID, serverName, serverURL sql.NullString
	var lastUsed sql.NullTime
	err := row.Scan(&k.ID, &k.Key, &k.Name, &k.KeyType, &isActive,
		&machineID, &lastUsed, &serverName, &serverURL, &k.CreatedAt)
	if err != nil {
		return nil, err
	}
	if s.secrets != nil {
		k.Key, err = s.secrets.Decrypt(k.Key)
		if err != nil {
			return nil, fmt.Errorf("decrypt API key %d: %w", k.ID, err)
		}
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

func (s *Store) scanAPIKeyRow(rows *sql.Rows) (*domain.APIKey, error) {
	var k domain.APIKey
	var isActive int
	var machineID, serverName, serverURL sql.NullString
	var lastUsed sql.NullTime
	err := rows.Scan(&k.ID, &k.Key, &k.Name, &k.KeyType, &isActive,
		&machineID, &lastUsed, &serverName, &serverURL, &k.CreatedAt)
	if err != nil {
		return nil, err
	}
	if s.secrets != nil {
		k.Key, err = s.secrets.Decrypt(k.Key)
		if err != nil {
			return nil, fmt.Errorf("decrypt API key %d: %w", k.ID, err)
		}
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

func (s *Store) protectAPIKeyValue(ctx context.Context, id int64, key string) error {
	encrypted, err := s.secrets.Encrypt(key)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE api_keys SET key=?, key_hash=? WHERE id=?`,
		encrypted, s.secrets.LookupHash(key), id)
	return err
}
