package store

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"time"

	"probakgo/internal/domain"
)

func (s *Store) GetAPIKeyByValue(key string) (*domain.APIKey, error) {
	row := s.db.QueryRow(`SELECT id, key, name, key_type, is_active, machine_id, last_used, server_name, created_at
		FROM api_keys WHERE key = ?`, key)
	return scanAPIKey(row)
}

func (s *Store) GetAPIKey(id int64) (*domain.APIKey, error) {
	row := s.db.QueryRow(`SELECT id, key, name, key_type, is_active, machine_id, last_used, server_name, created_at
		FROM api_keys WHERE id = ?`, id)
	return scanAPIKey(row)
}

func (s *Store) ListAPIKeys() ([]domain.APIKey, error) {
	rows, err := s.db.Query(`SELECT id, key, name, key_type, is_active, machine_id, last_used, server_name, created_at
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

func (s *Store) CreateAPIKey(name, keyType, serverName string) (*domain.APIKey, error) {
	prefix := map[string]string{
		"server": "pbk",
		"admin":  "adm",
	}[keyType]
	if prefix == "" {
		return nil, fmt.Errorf("unknown key_type: %s", keyType)
	}
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	key := fmt.Sprintf("%s-%x", prefix, raw)

	res, err := s.db.Exec(
		`INSERT INTO api_keys (key, name, key_type, server_name) VALUES (?, ?, ?, ?)`,
		key, name, keyType, serverName,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetAPIKey(id)
}

func (s *Store) UpdateAPIKeyLastUsed(id int64) error {
	_, err := s.db.Exec(`UPDATE api_keys SET last_used=? WHERE id=?`, time.Now(), id)
	return err
}

func (s *Store) BindAPIKeyMachineID(id int64, machineID string) error {
	_, err := s.db.Exec(`UPDATE api_keys SET machine_id=? WHERE id=?`, machineID, id)
	return err
}

func (s *Store) UnbindAPIKeyMachineID(id int64) error {
	_, err := s.db.Exec(`UPDATE api_keys SET machine_id='' WHERE id=?`, id)
	return err
}

func (s *Store) ToggleAPIKey(id int64) error {
	_, err := s.db.Exec(`UPDATE api_keys SET is_active = NOT is_active WHERE id=?`, id)
	return err
}

func (s *Store) UpdateAPIKey(id int64, name, serverName string) error {
	_, err := s.db.Exec(`UPDATE api_keys SET name=?, server_name=? WHERE id=?`, name, serverName, id)
	return err
}

func (s *Store) DeleteAPIKey(id int64) error {
	_, err := s.db.Exec(`DELETE FROM api_keys WHERE id=?`, id)
	return err
}

func (s *Store) HasAdminKey() (bool, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM api_keys WHERE key_type='admin'`).Scan(&count)
	return count > 0, err
}

func scanAPIKey(row *sql.Row) (*domain.APIKey, error) {
	var k domain.APIKey
	var isActive int
	var machineID, serverName sql.NullString
	var lastUsed sql.NullTime
	err := row.Scan(&k.ID, &k.Key, &k.Name, &k.KeyType, &isActive,
		&machineID, &lastUsed, &serverName, &k.CreatedAt)
	if err != nil {
		return nil, err
	}
	k.IsActive = isActive != 0
	k.MachineID = machineID.String
	k.ServerName = serverName.String
	if lastUsed.Valid {
		k.LastUsed = &lastUsed.Time
	}
	return &k, nil
}

func scanAPIKeyRow(rows *sql.Rows) (*domain.APIKey, error) {
	var k domain.APIKey
	var isActive int
	var machineID, serverName sql.NullString
	var lastUsed sql.NullTime
	err := rows.Scan(&k.ID, &k.Key, &k.Name, &k.KeyType, &isActive,
		&machineID, &lastUsed, &serverName, &k.CreatedAt)
	if err != nil {
		return nil, err
	}
	k.IsActive = isActive != 0
	k.MachineID = machineID.String
	k.ServerName = serverName.String
	if lastUsed.Valid {
		k.LastUsed = &lastUsed.Time
	}
	return &k, nil
}
