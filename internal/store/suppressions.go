package store

import (
	"context"
	"database/sql"
	"time"

	"probakgo/internal/debug"
)

type AlertSuppression struct {
	AlertID string
	Until   time.Time
	Reason  string
	Active  bool
}

func (s *Store) UpsertAlertSuppression(ctx context.Context, alertID string, until time.Time, reason string) error {
	debug.RecordQuery(ctx, `INSERT INTO alert_suppressions (alert_id, suppressed_until, reason) VALUES (?, ?, ?) ON CONFLICT(alert_id) DO UPDATE SET suppressed_until=excluded.suppressed_until, reason=excluded.reason`)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO alert_suppressions (alert_id, suppressed_until, reason)
		VALUES (?, ?, ?)
		ON CONFLICT(alert_id) DO UPDATE SET
			suppressed_until=excluded.suppressed_until,
			reason=excluded.reason`,
		alertID, until.Unix(), reason,
	)
	return err
}

func (s *Store) GetActiveSuppressions(ctx context.Context) (map[string]time.Time, error) {
	debug.RecordQuery(ctx, `SELECT alert_id, suppressed_until FROM alert_suppressions WHERE suppressed_until > ?`)
	rows, err := s.db.QueryContext(ctx,
		`SELECT alert_id, suppressed_until FROM alert_suppressions WHERE suppressed_until > ?`,
		time.Now().Unix(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]time.Time)
	for rows.Next() {
		var id string
		var until int64
		if err := rows.Scan(&id, &until); err != nil {
			return nil, err
		}
		result[id] = time.Unix(until, 0)
	}
	return result, rows.Err()
}

func (s *Store) DeleteAlertSuppression(ctx context.Context, alertID string) error {
	debug.RecordQuery(ctx, `DELETE FROM alert_suppressions WHERE alert_id = ?`)
	_, err := s.db.ExecContext(ctx, `DELETE FROM alert_suppressions WHERE alert_id = ?`, alertID)
	return err
}

func (s *Store) GetAlertSuppression(ctx context.Context, alertID string) (AlertSuppression, error) {
	debug.RecordQuery(ctx, `SELECT alert_id, suppressed_until, reason FROM alert_suppressions WHERE alert_id = ?`)
	row := s.db.QueryRowContext(ctx,
		`SELECT alert_id, suppressed_until, reason FROM alert_suppressions WHERE alert_id = ?`,
		alertID,
	)
	var sup AlertSuppression
	var until int64
	if err := row.Scan(&sup.AlertID, &until, &sup.Reason); err != nil {
		if err == sql.ErrNoRows {
			return AlertSuppression{}, err
		}
		return AlertSuppression{}, err
	}
	sup.Until = time.Unix(until, 0)
	sup.Active = sup.Until.After(time.Now())
	return sup, nil
}
