package store

import (
	"context"
	"database/sql"
	"strings"

	"probakgo/internal/debug"
	"probakgo/internal/domain"
)

func (s *Store) GetEmailConfig(ctx context.Context) (*domain.EmailConfig, error) {
	debug.RecordQuery(ctx, `SELECT id, smtp_host, smtp_port, smtp_user, smtp_password, recipients, is_enabled, send_time, retention_months, retention_enabled, alert_disk_pct, alert_windows_disk_pct, alert_backup_err, alert_pbs_stale_hours, public_api_url, vpn_only_access, alert_pve_heartbeat_minutes, critical_alerts_enabled, enforce_totp_non_readers, sensitive_actions_require_totp FROM email_config LIMIT 1`)
	row := s.db.QueryRowContext(ctx, `
		SELECT id, smtp_host, smtp_port, smtp_user, smtp_password, recipients,
		       is_enabled, send_time,
		       retention_months, retention_enabled, alert_disk_pct, alert_windows_disk_pct, alert_backup_err,
		       alert_pbs_stale_hours, public_api_url, vpn_only_access, alert_pve_heartbeat_minutes,
		       critical_alerts_enabled, enforce_totp_non_readers, sensitive_actions_require_totp
		FROM email_config LIMIT 1`)
	var c domain.EmailConfig
	var isEnabled, retEnabled, alertBackupErr, vpnOnlyAccess, criticalAlertsEnabled, enforceTOTPNonReaders, sensitiveActionsRequireTOTP int
	err := row.Scan(
		&c.ID, &c.SMTPHost, &c.SMTPPort, &c.SMTPUser, &c.SMTPPass,
		&c.Recipients, &isEnabled, &c.SendTime,
		&c.RetentionMonths, &retEnabled, &c.AlertDiskPct, &c.AlertWindowsDiskPct, &alertBackupErr,
		&c.AlertPBSStaleHours, &c.PublicAPIURL, &vpnOnlyAccess, &c.AlertPVEHeartbeatMinutes,
		&criticalAlertsEnabled, &enforceTOTPNonReaders, &sensitiveActionsRequireTOTP,
	)
	if err == sql.ErrNoRows {
		return &domain.EmailConfig{
			SendTime:                 "08:00",
			RetentionMonths:          3,
			RetentionEnabled:         true,
			AlertDiskPct:             85,
			AlertWindowsDiskPct:      90,
			AlertBackupErr:           true,
			AlertPBSStaleHours:       48,
			AlertPVEHeartbeatMinutes: 15,
		}, nil
	}
	if err != nil {
		return nil, err
	}
	c.IsEnabled = isEnabled != 0
	c.RetentionEnabled = retEnabled != 0
	c.AlertBackupErr = alertBackupErr != 0
	c.VPNOnlyAccess = vpnOnlyAccess != 0
	c.CriticalAlertsEnabled = criticalAlertsEnabled != 0
	c.EnforceTOTPNonReaders = enforceTOTPNonReaders != 0
	c.SensitiveActionsRequireTOTP = sensitiveActionsRequireTOTP != 0
	return &c, nil
}

func (s *Store) UpsertEmailConfig(ctx context.Context, c domain.EmailConfig) error {
	debug.RecordQuery(ctx, `INSERT INTO email_config (id, smtp_host, smtp_port, smtp_user, smtp_password, recipients, is_enabled, send_time, retention_months, retention_enabled, alert_disk_pct, alert_windows_disk_pct, alert_backup_err, alert_pbs_stale_hours, public_api_url, vpn_only_access, alert_pve_heartbeat_minutes, critical_alerts_enabled, enforce_totp_non_readers, sensitive_actions_require_totp) VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(id) DO UPDATE SET ...`)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO email_config (
			id, smtp_host, smtp_port, smtp_user, smtp_password, recipients,
			is_enabled, send_time,
			retention_months, retention_enabled, alert_disk_pct, alert_windows_disk_pct, alert_backup_err,
			alert_pbs_stale_hours, public_api_url, vpn_only_access, alert_pve_heartbeat_minutes,
			critical_alerts_enabled, enforce_totp_non_readers, sensitive_actions_require_totp
		) VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			smtp_host=excluded.smtp_host,
			smtp_port=excluded.smtp_port,
			smtp_user=excluded.smtp_user,
			smtp_password=excluded.smtp_password,
			recipients=excluded.recipients,
			is_enabled=excluded.is_enabled,
			send_time=excluded.send_time,
			retention_months=excluded.retention_months,
			retention_enabled=excluded.retention_enabled,
			alert_disk_pct=excluded.alert_disk_pct,
			alert_windows_disk_pct=excluded.alert_windows_disk_pct,
			alert_backup_err=excluded.alert_backup_err,
			alert_pbs_stale_hours=excluded.alert_pbs_stale_hours,
			public_api_url=excluded.public_api_url,
			vpn_only_access=excluded.vpn_only_access,
			alert_pve_heartbeat_minutes=excluded.alert_pve_heartbeat_minutes,
			critical_alerts_enabled=excluded.critical_alerts_enabled,
			enforce_totp_non_readers=excluded.enforce_totp_non_readers,
			sensitive_actions_require_totp=excluded.sensitive_actions_require_totp`,
		c.SMTPHost, c.SMTPPort, c.SMTPUser, c.SMTPPass,
		c.Recipients, boolToInt(c.IsEnabled), c.SendTime,
		c.RetentionMonths, boolToInt(c.RetentionEnabled), c.AlertDiskPct, c.AlertWindowsDiskPct, boolToInt(c.AlertBackupErr),
		c.AlertPBSStaleHours, c.PublicAPIURL, boolToInt(c.VPNOnlyAccess), c.AlertPVEHeartbeatMinutes,
		boolToInt(c.CriticalAlertsEnabled), boolToInt(c.EnforceTOTPNonReaders), boolToInt(c.SensitiveActionsRequireTOTP),
	)
	return err
}

func (s *Store) GetEmailDeliveryStatus(ctx context.Context) (*domain.EmailDeliveryStatus, error) {
	debug.RecordQuery(ctx, `SELECT last_attempt_at, last_success_at, last_error FROM email_delivery_status WHERE id = 1`)
	row := s.db.QueryRowContext(ctx, `SELECT last_attempt_at, last_success_at, last_error FROM email_delivery_status WHERE id = 1`)
	var lastAttempt sql.NullTime
	var lastSuccess sql.NullTime
	var lastError sql.NullString
	if err := row.Scan(&lastAttempt, &lastSuccess, &lastError); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	status := &domain.EmailDeliveryStatus{LastError: lastError.String}
	if lastAttempt.Valid {
		status.LastAttemptAt = &lastAttempt.Time
	}
	if lastSuccess.Valid {
		status.LastSuccessAt = &lastSuccess.Time
	}
	return status, nil
}

func (s *Store) RecordEmailDelivery(ctx context.Context, err error) error {
	lastError := ""
	if err != nil {
		lastError = strings.TrimSpace(err.Error())
	}
	if lastError == "" {
		debug.RecordQuery(ctx, `INSERT INTO email_delivery_status (id, last_attempt_at, last_success_at, last_error) VALUES (1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, '') ON CONFLICT(id) DO UPDATE SET ...`)
		_, execErr := s.db.ExecContext(ctx, `
			INSERT INTO email_delivery_status (id, last_attempt_at, last_success_at, last_error)
			VALUES (1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, '')
			ON CONFLICT(id) DO UPDATE SET
				last_attempt_at=CURRENT_TIMESTAMP,
				last_success_at=CURRENT_TIMESTAMP,
				last_error=''`)
		return execErr
	}
	debug.RecordQuery(ctx, `INSERT INTO email_delivery_status (id, last_attempt_at, last_error) VALUES (1, CURRENT_TIMESTAMP, ?) ON CONFLICT(id) DO UPDATE SET ...`)
	_, execErr := s.db.ExecContext(ctx, `
		INSERT INTO email_delivery_status (id, last_attempt_at, last_error)
		VALUES (1, CURRENT_TIMESTAMP, ?)
		ON CONFLICT(id) DO UPDATE SET
			last_attempt_at=CURRENT_TIMESTAMP,
			last_error=excluded.last_error`, lastError)
	return execErr
}
