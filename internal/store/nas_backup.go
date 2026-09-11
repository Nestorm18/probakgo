package store

import (
	"context"
	"fmt"
	"probakgo/internal/domain"
)

func (s *Store) GetNASBackupConfig(ctx context.Context) (*domain.NASBackupConfig, error) {
	c := &domain.NASBackupConfig{}
	err := s.db.QueryRowContext(ctx, `SELECT enabled, host, port, username, password, directory, send_time, last_attempt, last_success, last_error, last_scheduled_attempt FROM nas_backup_config WHERE id=1`).Scan(&c.Enabled, &c.Host, &c.Port, &c.Username, &c.Password, &c.Directory, &c.SendTime, &c.LastAttempt, &c.LastSuccess, &c.LastError, &c.LastScheduledAttempt)
	if err != nil {
		return nil, err
	}
	if c.Password != "" {
		if s.secrets == nil {
			return nil, fmt.Errorf("NAS credentials require DATA_ENCRYPTION_KEY")
		}
		c.Password, err = s.secrets.Decrypt(c.Password)
	}
	return c, err
}

func (s *Store) SaveNASBackupConfig(ctx context.Context, c domain.NASBackupConfig) error {
	password := ""
	if c.Password != "" {
		if s.secrets == nil {
			return fmt.Errorf("NAS credentials require DATA_ENCRYPTION_KEY")
		}
		var err error
		password, err = s.secrets.Encrypt(c.Password)
		if err != nil {
			return err
		}
	}
	_, err := s.db.ExecContext(ctx, `UPDATE nas_backup_config SET enabled=?, host=?, port=?, username=?, password=?, directory=?, send_time=? WHERE id=1`, c.Enabled, c.Host, c.Port, c.Username, password, c.Directory, c.SendTime)
	return err
}

// Claim the day's run before uploading, so restarting cannot duplicate it.
func (s *Store) ClaimNASBackup(ctx context.Context, previous, attempt string) (bool, error) {
	return s.claimNASBackup(ctx, previous, attempt, false)
}

func (s *Store) ClaimManualNASBackup(ctx context.Context, previous, attempt string) (bool, error) {
	return s.claimNASBackup(ctx, previous, attempt, true)
}

func (s *Store) claimNASBackup(ctx context.Context, previous, attempt string, manual bool) (bool, error) {
	res, err := s.db.ExecContext(ctx, `UPDATE nas_backup_config SET last_attempt=?, last_scheduled_attempt=CASE WHEN ? THEN last_scheduled_attempt ELSE ? END, last_error='Copia en curso; si el servicio se reinicia, se retomará al día siguiente' WHERE id=1 AND (enabled=1 OR ?) AND last_attempt=?`, attempt, manual, attempt, manual, previous)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

func (s *Store) FinishNASBackup(ctx context.Context, attempt, failure string) error {
	return s.FinishNASBackupResult(ctx, attempt, failure == "", failure)
}

func (s *Store) FinishNASBackupResult(ctx context.Context, attempt string, completed bool, message string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE nas_backup_config SET last_error=?, last_success=CASE WHEN ? THEN ? ELSE last_success END WHERE id=1 AND last_attempt=?`, message, completed, attempt, attempt)
	return err
}
