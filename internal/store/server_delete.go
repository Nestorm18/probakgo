package store

import (
	"context"
	"database/sql"
	"fmt"
)

// HardDeleteServerData permanently deletes all data for the server with the given name
// from both PVE and PBS tables. It is a no-op if the server does not exist in either table.
func (s *Store) HardDeleteServerData(ctx context.Context, serverName string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if err := hardDeletePVE(ctx, tx, serverName); err != nil {
		return err
	}
	if err := hardDeletePBS(ctx, tx, serverName); err != nil {
		return err
	}

	return tx.Commit()
}

func hardDeletePVE(ctx context.Context, tx interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, serverName string) error {
	var id int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM pve_servers WHERE name = ?`, serverName).Scan(&id); err != nil {
		return nil // not found, nothing to do
	}

	steps := []string{
		`DELETE FROM server_heartbeats WHERE server_type = 'pve' AND server_id = ?`,
		`DELETE FROM pve_vm_alert_config WHERE server_id = ?`,
		`DELETE FROM pve_alert_config WHERE server_id = ?`,
		`DELETE FROM pve_backup_tasks WHERE report_id IN (SELECT id FROM pve_reports WHERE server_id = ?)`,
		`DELETE FROM pve_storage_content WHERE storage_id IN (SELECT id FROM pve_storages WHERE report_id IN (SELECT id FROM pve_reports WHERE server_id = ?))`,
		`DELETE FROM pve_storage_info WHERE storage_id IN (SELECT id FROM pve_storages WHERE report_id IN (SELECT id FROM pve_reports WHERE server_id = ?))`,
		`DELETE FROM pve_storages WHERE report_id IN (SELECT id FROM pve_reports WHERE server_id = ?)`,
		`DELETE FROM pve_reports WHERE server_id = ?`,
		`DELETE FROM pve_servers WHERE id = ?`,
	}
	for _, stmt := range steps {
		if _, err := tx.ExecContext(ctx, stmt, id); err != nil {
			return fmt.Errorf("pve delete %q: %w", stmt, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM vm_backup_configs WHERE server_name = ?`, serverName); err != nil {
		return fmt.Errorf("vm_backup_configs delete: %w", err)
	}
	return nil
}

func hardDeletePBS(ctx context.Context, tx interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, serverName string) error {
	var id int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM pbs_servers WHERE name = ?`, serverName).Scan(&id); err != nil {
		return nil // not found, nothing to do
	}

	steps := []string{
		`DELETE FROM server_heartbeats WHERE server_type = 'pbs' AND server_id = ?`,
		`DELETE FROM pbs_alert_config WHERE server_id = ?`,
		`DELETE FROM pbs_snapshots WHERE store_id IN (SELECT id FROM pbs_stores WHERE report_id IN (SELECT id FROM pbs_reports WHERE server_id = ?))`,
		`DELETE FROM pbs_store_history WHERE store_id IN (SELECT id FROM pbs_stores WHERE report_id IN (SELECT id FROM pbs_reports WHERE server_id = ?))`,
		`DELETE FROM pbs_gc_status WHERE store_id IN (SELECT id FROM pbs_stores WHERE report_id IN (SELECT id FROM pbs_reports WHERE server_id = ?))`,
		`DELETE FROM pbs_stores WHERE report_id IN (SELECT id FROM pbs_reports WHERE server_id = ?)`,
		`DELETE FROM pbs_reports WHERE server_id = ?`,
		`DELETE FROM pbs_servers WHERE id = ?`,
	}
	for _, stmt := range steps {
		if _, err := tx.ExecContext(ctx, stmt, id); err != nil {
			return fmt.Errorf("pbs delete %q: %w", stmt, err)
		}
	}
	return nil
}
