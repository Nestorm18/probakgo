package store

import (
	"context"
	"fmt"
)

// ResetAllData deletes all operational data. Users are preserved.
func (s *Store) ResetAllData(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	tables := []string{
		"pve_vm_alert_config",
		"pve_alert_config",
		"pbs_alert_config",
		"alert_suppressions",
		"pve_backup_tasks",
		"pve_storage_content",
		"pve_storage_info",
		"pve_storages",
		"pve_reports",
		"pve_servers",
		"pbs_snapshots",
		"pbs_store_history",
		"pbs_gc_status",
		"pbs_stores",
		"pbs_reports",
		"pbs_servers",
		"api_keys",
		"vm_backup_configs",
		"email_config",
		"ip_bans",
		"login_attempts",
	}
	for _, table := range tables {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table); err != nil {
			return fmt.Errorf("reset %s: %w", table, err)
		}
	}

	return tx.Commit()
}
