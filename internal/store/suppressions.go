package store

import (
	"time"
)

func (s *Store) UpsertAlertSuppression(alertID string, until time.Time, reason string) error {
	_, err := s.db.Exec(`
		INSERT INTO alert_suppressions (alert_id, suppressed_until, reason)
		VALUES (?, ?, ?)
		ON CONFLICT(alert_id) DO UPDATE SET
			suppressed_until=excluded.suppressed_until,
			reason=excluded.reason`,
		alertID, until.Unix(), reason,
	)
	return err
}

func (s *Store) GetActiveSuppressions() (map[string]time.Time, error) {
	rows, err := s.db.Query(
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

func (s *Store) DeleteAlertSuppression(alertID string) error {
	_, err := s.db.Exec(`DELETE FROM alert_suppressions WHERE alert_id = ?`, alertID)
	return err
}
