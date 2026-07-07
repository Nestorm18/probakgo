package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"probakgo/internal/debug"
	"probakgo/internal/domain"
)

func ServerMaintenanceKey(serverType string, serverID int64) string {
	return fmt.Sprintf("%s:%d", serverType, serverID)
}

func (s *Store) UpsertServerMaintenance(ctx context.Context, serverType string, serverID int64, until time.Time, reason string) error {
	serverType = strings.TrimSpace(serverType)
	reason = strings.TrimSpace(reason)
	debug.RecordQuery(ctx, `INSERT INTO server_maintenance (server_type, server_id, maintenance_until, reason, updated_at) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP) ON CONFLICT(server_type, server_id) DO UPDATE SET ...`)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO server_maintenance (server_type, server_id, maintenance_until, reason, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(server_type, server_id) DO UPDATE SET
			maintenance_until=excluded.maintenance_until,
			reason=excluded.reason,
			updated_at=CURRENT_TIMESTAMP`,
		serverType, serverID, until.Unix(), reason,
	)
	return err
}

func (s *Store) DeleteServerMaintenance(ctx context.Context, serverType string, serverID int64) error {
	debug.RecordQuery(ctx, `DELETE FROM server_maintenance WHERE server_type = ? AND server_id = ?`)
	_, err := s.db.ExecContext(ctx, `DELETE FROM server_maintenance WHERE server_type = ? AND server_id = ?`, serverType, serverID)
	return err
}

func (s *Store) GetServerMaintenance(ctx context.Context, serverType string, serverID int64) (domain.ServerMaintenance, error) {
	debug.RecordQuery(ctx, `SELECT server_type, server_id, maintenance_until, reason FROM server_maintenance WHERE server_type = ? AND server_id = ?`)
	row := s.db.QueryRowContext(ctx, `
		SELECT server_type, server_id, maintenance_until, reason
		FROM server_maintenance
		WHERE server_type = ? AND server_id = ?`, serverType, serverID)
	return scanServerMaintenance(row)
}

func (s *Store) GetActiveServerMaintenances(ctx context.Context) (map[string]domain.ServerMaintenance, error) {
	debug.RecordQuery(ctx, `SELECT server_type, server_id, maintenance_until, reason FROM server_maintenance WHERE maintenance_until > ?`)
	rows, err := s.db.QueryContext(ctx, `
		SELECT server_type, server_id, maintenance_until, reason
		FROM server_maintenance
		WHERE maintenance_until > ?`, time.Now().Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]domain.ServerMaintenance)
	for rows.Next() {
		m, err := scanServerMaintenanceRows(rows)
		if err != nil {
			return nil, err
		}
		if m.Active {
			out[ServerMaintenanceKey(m.ServerType, m.ServerID)] = m
		}
	}
	return out, rows.Err()
}

type maintenanceScanner interface {
	Scan(dest ...any) error
}

func scanServerMaintenance(row maintenanceScanner) (domain.ServerMaintenance, error) {
	var m domain.ServerMaintenance
	var until int64
	if err := row.Scan(&m.ServerType, &m.ServerID, &until, &m.Reason); err != nil {
		if err == sql.ErrNoRows {
			return domain.ServerMaintenance{}, err
		}
		return domain.ServerMaintenance{}, err
	}
	m.Until = time.Unix(until, 0)
	m.Active = m.Until.After(time.Now())
	return m, nil
}

func scanServerMaintenanceRows(rows *sql.Rows) (domain.ServerMaintenance, error) {
	return scanServerMaintenance(rows)
}
