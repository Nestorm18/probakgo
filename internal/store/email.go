package store

import (
	"database/sql"

	"probakgo/internal/domain"
)

func (s *Store) GetEmailConfig() (*domain.EmailConfig, error) {
	row := s.db.QueryRow(`
		SELECT id, smtp_host, smtp_port, smtp_user, smtp_password, recipients,
		       is_enabled, send_time,
		       retention_months, retention_enabled, alert_disk_pct, alert_backup_err,
		       alert_pbs_stale_hours
		FROM email_config LIMIT 1`)
	var c domain.EmailConfig
	var isEnabled, retEnabled, alertBackupErr int
	err := row.Scan(
		&c.ID, &c.SMTPHost, &c.SMTPPort, &c.SMTPUser, &c.SMTPPass,
		&c.Recipients, &isEnabled, &c.SendTime,
		&c.RetentionMonths, &retEnabled, &c.AlertDiskPct, &alertBackupErr,
		&c.AlertPBSStaleHours,
	)
	if err == sql.ErrNoRows {
		return &domain.EmailConfig{
			SendTime:           "08:00",
			RetentionMonths:    3,
			RetentionEnabled:   true,
			AlertDiskPct:       85,
			AlertBackupErr:     true,
			AlertPBSStaleHours: 48,
		}, nil
	}
	if err != nil {
		return nil, err
	}
	c.IsEnabled = isEnabled != 0
	c.RetentionEnabled = retEnabled != 0
	c.AlertBackupErr = alertBackupErr != 0
	return &c, nil
}

func (s *Store) UpsertEmailConfig(c domain.EmailConfig) error {
	_, err := s.db.Exec(`
		INSERT INTO email_config (
			id, smtp_host, smtp_port, smtp_user, smtp_password, recipients,
			is_enabled, send_time,
			retention_months, retention_enabled, alert_disk_pct, alert_backup_err,
			alert_pbs_stale_hours
		) VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
			alert_backup_err=excluded.alert_backup_err,
			alert_pbs_stale_hours=excluded.alert_pbs_stale_hours`,
		c.SMTPHost, c.SMTPPort, c.SMTPUser, c.SMTPPass,
		c.Recipients, boolToInt(c.IsEnabled), c.SendTime,
		c.RetentionMonths, boolToInt(c.RetentionEnabled), c.AlertDiskPct, boolToInt(c.AlertBackupErr),
		c.AlertPBSStaleHours,
	)
	return err
}
