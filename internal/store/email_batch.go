package store

import (
	"context"
	"time"
)

// AlertEmailBatchDue persists the first pending update across server restarts.
func (s *Store) AlertEmailBatchDue(ctx context.Context, minutes int, pending bool, now time.Time) (bool, error) {
	if !pending || minutes == 0 {
		_, err := s.db.ExecContext(ctx, `DELETE FROM alert_email_batch`)
		return pending, err
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO alert_email_batch (id, pending_since) VALUES (1, ?) ON CONFLICT(id) DO NOTHING`, now.Unix()); err != nil {
		return false, err
	}
	var since int64
	if err := s.db.QueryRowContext(ctx, `SELECT pending_since FROM alert_email_batch WHERE id = 1`).Scan(&since); err != nil {
		return false, err
	}
	return !now.Before(time.Unix(since, 0).Add(time.Duration(minutes) * time.Minute)), nil
}

func (s *Store) ClearAlertEmailBatch(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM alert_email_batch`)
	return err
}
