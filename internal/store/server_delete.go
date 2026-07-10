package store

import (
	"context"
	"database/sql"
	"fmt"
)

type txRunner interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

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
	if err := hardDeleteWindows(ctx, tx, serverName); err != nil {
		return err
	}

	return tx.Commit()
}

// HardDeleteServerDataForAPIKey deletes the server bound to an API key. The
// fallback name is only used for legacy rows that predate api_key_id.
func (s *Store) HardDeleteServerDataForAPIKey(ctx context.Context, apiKeyID int64, fallbackName string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	deletedPVE, err := hardDeletePVEByAPIKey(ctx, tx, apiKeyID)
	if err != nil {
		return err
	}
	deletedPBS, err := hardDeletePBSByAPIKey(ctx, tx, apiKeyID)
	if err != nil {
		return err
	}
	deletedWindows, err := hardDeleteWindowsByAPIKey(ctx, tx, apiKeyID)
	if err != nil {
		return err
	}
	if !deletedPVE && !deletedPBS && !deletedWindows && fallbackName != "" {
		if err := hardDeletePVE(ctx, tx, fallbackName); err != nil {
			return err
		}
		if err := hardDeletePBS(ctx, tx, fallbackName); err != nil {
			return err
		}
		if err := hardDeleteWindows(ctx, tx, fallbackName); err != nil {
			return err
		}
	} else if fallbackName != "" {
		if err := hardDeleteLegacyPVE(ctx, tx, fallbackName); err != nil {
			return err
		}
		if err := hardDeleteLegacyPBS(ctx, tx, fallbackName); err != nil {
			return err
		}
		if err := hardDeleteLegacyWindows(ctx, tx, fallbackName); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func hardDeletePVE(ctx context.Context, tx txRunner, serverName string) error {
	ids, err := pveServerIDsByName(ctx, tx, serverName)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := hardDeletePVEByID(ctx, tx, id); err != nil {
			return err
		}
	}
	return nil
}

func hardDeletePVEByAPIKey(ctx context.Context, tx txRunner, apiKeyID int64) (bool, error) {
	ids, err := pveServerIDsByAPIKey(ctx, tx, apiKeyID)
	if err != nil {
		return false, err
	}
	for _, id := range ids {
		if err := hardDeletePVEByID(ctx, tx, id); err != nil {
			return false, err
		}
	}
	return len(ids) > 0, nil
}

func hardDeleteLegacyPVE(ctx context.Context, tx txRunner, serverName string) error {
	ids, err := legacyPVEServerIDsByName(ctx, tx, serverName)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := hardDeletePVEByID(ctx, tx, id); err != nil {
			return err
		}
	}
	return nil
}

func hardDeletePVEByID(ctx context.Context, tx txRunner, id int64) error {
	steps := []string{
		`DELETE FROM server_heartbeats WHERE server_type = 'pve' AND server_id = ?`,
		`DELETE FROM server_maintenance WHERE server_type = 'pve' AND server_id = ?`,
		`DELETE FROM pve_vm_alert_config WHERE server_id = ?`,
		`DELETE FROM pve_alert_config WHERE server_id = ?`,
		`DELETE FROM vm_backup_configs WHERE server_type = 'pve' AND server_id = ?`,
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
	if err := deleteAlertSuppressionsForServer(ctx, tx, "pve", id); err != nil {
		return err
	}
	return nil
}

func hardDeletePBS(ctx context.Context, tx txRunner, serverName string) error {
	ids, err := pbsServerIDsByName(ctx, tx, serverName)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := hardDeletePBSByID(ctx, tx, id); err != nil {
			return err
		}
	}
	return nil
}

func hardDeletePBSByAPIKey(ctx context.Context, tx txRunner, apiKeyID int64) (bool, error) {
	ids, err := pbsServerIDsByAPIKey(ctx, tx, apiKeyID)
	if err != nil {
		return false, err
	}
	for _, id := range ids {
		if err := hardDeletePBSByID(ctx, tx, id); err != nil {
			return false, err
		}
	}
	return len(ids) > 0, nil
}

func hardDeleteLegacyPBS(ctx context.Context, tx txRunner, serverName string) error {
	ids, err := legacyPBSServerIDsByName(ctx, tx, serverName)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := hardDeletePBSByID(ctx, tx, id); err != nil {
			return err
		}
	}
	return nil
}

func hardDeletePBSByID(ctx context.Context, tx txRunner, id int64) error {
	steps := []string{
		`DELETE FROM server_heartbeats WHERE server_type = 'pbs' AND server_id = ?`,
		`DELETE FROM server_maintenance WHERE server_type = 'pbs' AND server_id = ?`,
		`DELETE FROM pbs_alert_config WHERE server_id = ?`,
		`DELETE FROM pbs_maintenance_tasks WHERE report_id IN (SELECT id FROM pbs_reports WHERE server_id = ?)`,
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
	if err := deleteAlertSuppressionsForServer(ctx, tx, "pbs", id); err != nil {
		return err
	}
	return nil
}

func pveServerIDsByName(ctx context.Context, tx txRunner, serverName string) ([]int64, error) {
	return serverIDs(ctx, tx, `SELECT id FROM pve_servers WHERE name = ?`, serverName)
}

func pveServerIDsByAPIKey(ctx context.Context, tx txRunner, apiKeyID int64) ([]int64, error) {
	return serverIDs(ctx, tx, `SELECT id FROM pve_servers WHERE api_key_id = ?`, apiKeyID)
}

func legacyPVEServerIDsByName(ctx context.Context, tx txRunner, serverName string) ([]int64, error) {
	return serverIDs(ctx, tx, `SELECT id FROM pve_servers WHERE name = ? AND api_key_id IS NULL`, serverName)
}

func pbsServerIDsByName(ctx context.Context, tx txRunner, serverName string) ([]int64, error) {
	return serverIDs(ctx, tx, `SELECT id FROM pbs_servers WHERE name = ?`, serverName)
}

func pbsServerIDsByAPIKey(ctx context.Context, tx txRunner, apiKeyID int64) ([]int64, error) {
	return serverIDs(ctx, tx, `SELECT id FROM pbs_servers WHERE api_key_id = ?`, apiKeyID)
}

func legacyPBSServerIDsByName(ctx context.Context, tx txRunner, serverName string) ([]int64, error) {
	return serverIDs(ctx, tx, `SELECT id FROM pbs_servers WHERE name = ? AND api_key_id IS NULL`, serverName)
}

func hardDeleteWindows(ctx context.Context, tx txRunner, serverName string) error {
	ids, err := windowsServerIDsByName(ctx, tx, serverName)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := hardDeleteWindowsByID(ctx, tx, id); err != nil {
			return err
		}
	}
	return nil
}

func hardDeleteWindowsByAPIKey(ctx context.Context, tx txRunner, apiKeyID int64) (bool, error) {
	ids, err := windowsServerIDsByAPIKey(ctx, tx, apiKeyID)
	if err != nil {
		return false, err
	}
	for _, id := range ids {
		if err := hardDeleteWindowsByID(ctx, tx, id); err != nil {
			return false, err
		}
	}
	return len(ids) > 0, nil
}

func hardDeleteLegacyWindows(ctx context.Context, tx txRunner, serverName string) error {
	ids, err := legacyWindowsServerIDsByName(ctx, tx, serverName)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := hardDeleteWindowsByID(ctx, tx, id); err != nil {
			return err
		}
	}
	return nil
}

func hardDeleteWindowsByID(ctx context.Context, tx txRunner, id int64) error {
	steps := []string{
		`DELETE FROM server_heartbeats WHERE server_type = 'windows' AND server_id = ?`,
		`DELETE FROM server_maintenance WHERE server_type = 'windows' AND server_id = ?`,
		`DELETE FROM windows_alert_config WHERE server_id = ?`,
		`DELETE FROM windows_disks WHERE report_id IN (SELECT id FROM windows_reports WHERE server_id = ?)`,
		`DELETE FROM windows_reports WHERE server_id = ?`,
		`DELETE FROM windows_servers WHERE id = ?`,
	}
	for _, stmt := range steps {
		if _, err := tx.ExecContext(ctx, stmt, id); err != nil {
			return fmt.Errorf("windows delete %q: %w", stmt, err)
		}
	}
	if err := deleteAlertSuppressionsForServer(ctx, tx, "windows", id); err != nil {
		return err
	}
	return nil
}

func windowsServerIDsByName(ctx context.Context, tx txRunner, serverName string) ([]int64, error) {
	return serverIDs(ctx, tx, `SELECT id FROM windows_servers WHERE name = ?`, serverName)
}

func windowsServerIDsByAPIKey(ctx context.Context, tx txRunner, apiKeyID int64) ([]int64, error) {
	return serverIDs(ctx, tx, `SELECT id FROM windows_servers WHERE api_key_id = ?`, apiKeyID)
}

func legacyWindowsServerIDsByName(ctx context.Context, tx txRunner, serverName string) ([]int64, error) {
	return serverIDs(ctx, tx, `SELECT id FROM windows_servers WHERE name = ? AND api_key_id IS NULL`, serverName)
}

func serverIDs(ctx context.Context, tx txRunner, query string, arg any) ([]int64, error) {
	rows, err := tx.QueryContext(ctx, query, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func deleteAlertSuppressionsForServer(ctx context.Context, tx txRunner, serverType string, serverID int64) error {
	exact := fmt.Sprintf("%%:%s:%d", serverType, serverID)
	prefix := exact + ":%"
	if _, err := tx.ExecContext(ctx, `DELETE FROM alert_suppressions WHERE alert_id LIKE ? OR alert_id LIKE ?`, exact, prefix); err != nil {
		return fmt.Errorf("alert_suppressions delete: %w", err)
	}
	return nil
}
