package store

import (
	"context"
	"database/sql"
	"fmt"
)

// ResetAllData deletes all operational data. Users, audit logs and migration
// history are preserved so administrators retain access and a security trail.
// The security policy (2FA enforcement, TOTP for sensitive actions and VPN-only
// access) survives the reset; the rest of email_config returns to defaults.
func (s *Store) ResetAllData(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var enforceTOTP, sensitiveTOTP, vpnOnly bool
	err = tx.QueryRowContext(ctx, `SELECT enforce_totp_non_readers, sensitive_actions_require_totp, vpn_only_access
		FROM email_config LIMIT 1`).Scan(&enforceTOTP, &sensitiveTOTP, &vpnOnly)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("read security policy: %w", err)
	}

	tables := []string{
		"pve_vm_alert_config",
		"pve_alert_config",
		"pbs_alert_config",
		"windows_alert_config",
		"alert_state_events",
		"alert_states",
		"alert_email_batch",
		"alert_suppressions",
		"server_maintenance",
		"server_heartbeats",
		"pve_backup_tasks",
		"pve_storage_content",
		"pve_storage_info",
		"pve_storages",
		"pve_reports",
		"pve_servers",
		"pbs_maintenance_tasks",
		"pbs_snapshots",
		"pbs_store_history",
		"pbs_gc_status",
		"pbs_stores",
		"pbs_reports",
		"pbs_servers",
		"windows_disks",
		"windows_reports",
		"windows_servers",
		"api_keys",
		"vm_backup_configs",
		"email_delivery_status",
		"telegram_delivery_status",
		"telegram_alert_deliveries",
		"telegram_destinations",
		"telegram_config",
		"email_config",
		"nas_backup_config",
		"ip_bans",
		"login_attempts",
	}
	for _, table := range tables {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table); err != nil {
			return fmt.Errorf("reset %s: %w", table, err)
		}
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO nas_backup_config (id) VALUES (1)`); err != nil {
		return fmt.Errorf("reset NAS backup config: %w", err)
	}
	if enforceTOTP || sensitiveTOTP || vpnOnly {
		cfg := defaultEmailConfig()
		cfg.EnforceTOTPNonReaders = enforceTOTP
		cfg.SensitiveActionsRequireTOTP = sensitiveTOTP
		cfg.VPNOnlyAccess = vpnOnly
		if err := s.upsertEmailConfig(ctx, tx, *cfg); err != nil {
			return fmt.Errorf("restore security policy: %w", err)
		}
	}
	return tx.Commit()
}
