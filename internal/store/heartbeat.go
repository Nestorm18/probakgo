package store

import (
	"context"
	"time"

	"probakgo/internal/debug"
	"probakgo/internal/domain"
)

func (s *Store) UpsertServerHeartbeat(ctx context.Context, hb domain.ServerHeartbeat) error {
	if hb.LastSeenAt.IsZero() {
		hb.LastSeenAt = time.Now()
	}
	debug.RecordQuery(ctx, `INSERT INTO server_heartbeats (server_type, server_id, hostname, ip, public_ip, client_version, machine_id, last_seen_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(server_type, server_id) DO UPDATE SET ...`)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO server_heartbeats (
			server_type, server_id, hostname, ip, public_ip, client_version, machine_id, last_seen_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(server_type, server_id) DO UPDATE SET
			hostname=excluded.hostname,
			ip=excluded.ip,
			public_ip=excluded.public_ip,
			client_version=excluded.client_version,
			machine_id=excluded.machine_id,
			last_seen_at=excluded.last_seen_at,
			updated_at=CURRENT_TIMESTAMP`,
		hb.ServerType, hb.ServerID, hb.Hostname, hb.IP, hb.PublicIP,
		hb.ClientVersion, hb.MachineID, hb.LastSeenAt,
	)
	return err
}

func (s *Store) GetServerHeartbeat(ctx context.Context, serverType string, serverID int64) (*domain.ServerHeartbeat, error) {
	debug.RecordQuery(ctx, `SELECT id, server_type, server_id, hostname, ip, public_ip, client_version, machine_id, last_seen_at, created_at, updated_at FROM server_heartbeats WHERE server_type = ? AND server_id = ?`)
	row := s.db.QueryRowContext(ctx, `
		SELECT id, server_type, server_id, hostname, ip, public_ip,
		       client_version, machine_id, last_seen_at, created_at, updated_at
		FROM server_heartbeats
		WHERE server_type = ? AND server_id = ?`, serverType, serverID)
	var hb domain.ServerHeartbeat
	if err := row.Scan(&hb.ID, &hb.ServerType, &hb.ServerID, &hb.Hostname, &hb.IP, &hb.PublicIP,
		&hb.ClientVersion, &hb.MachineID, &hb.LastSeenAt, &hb.CreatedAt, &hb.UpdatedAt); err != nil {
		return nil, err
	}
	return &hb, nil
}

func (s *Store) ListServerHeartbeatsByType(ctx context.Context, serverType string) (map[int64]domain.ServerHeartbeat, error) {
	debug.RecordQuery(ctx, `SELECT id, server_type, server_id, hostname, ip, public_ip, client_version, machine_id, last_seen_at, created_at, updated_at FROM server_heartbeats WHERE server_type = ?`)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, server_type, server_id, hostname, ip, public_ip,
		       client_version, machine_id, last_seen_at, created_at, updated_at
		FROM server_heartbeats
		WHERE server_type = ?`, serverType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[int64]domain.ServerHeartbeat)
	for rows.Next() {
		var hb domain.ServerHeartbeat
		if err := rows.Scan(&hb.ID, &hb.ServerType, &hb.ServerID, &hb.Hostname, &hb.IP, &hb.PublicIP,
			&hb.ClientVersion, &hb.MachineID, &hb.LastSeenAt, &hb.CreatedAt, &hb.UpdatedAt); err != nil {
			return nil, err
		}
		result[hb.ServerID] = hb
	}
	return result, rows.Err()
}
