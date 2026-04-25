package store

import (
	"database/sql"

	"probaky/internal/domain"
)

func (s *Store) GetEmailConfig() (*domain.EmailConfig, error) {
	row := s.db.QueryRow(`SELECT id, smtp_host, smtp_port, smtp_user, smtp_password, recipients, is_enabled, send_time
		FROM email_config LIMIT 1`)
	var c domain.EmailConfig
	var isEnabled int
	err := row.Scan(&c.ID, &c.SMTPHost, &c.SMTPPort, &c.SMTPUser, &c.SMTPPass, &c.Recipients, &isEnabled, &c.SendTime)
	if err == sql.ErrNoRows {
		return &domain.EmailConfig{SendTime: "08:00"}, nil
	}
	if err != nil {
		return nil, err
	}
	c.IsEnabled = isEnabled != 0
	return &c, nil
}

func (s *Store) UpsertEmailConfig(c domain.EmailConfig) error {
	_, err := s.db.Exec(`
		INSERT INTO email_config (id, smtp_host, smtp_port, smtp_user, smtp_password, recipients, is_enabled, send_time)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			smtp_host=excluded.smtp_host,
			smtp_port=excluded.smtp_port,
			smtp_user=excluded.smtp_user,
			smtp_password=excluded.smtp_password,
			recipients=excluded.recipients,
			is_enabled=excluded.is_enabled,
			send_time=excluded.send_time`,
		c.SMTPHost, c.SMTPPort, c.SMTPUser, c.SMTPPass,
		c.Recipients, boolToInt(c.IsEnabled), c.SendTime,
	)
	return err
}
