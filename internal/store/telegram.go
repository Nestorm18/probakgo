package store

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"probakgo/internal/debug"
	"probakgo/internal/domain"
)

func (s *Store) GetTelegramConfig(ctx context.Context) (*domain.TelegramConfig, error) {
	debug.RecordQuery(ctx, `SELECT id, bot_token, bot_username, is_enabled, updated_at FROM telegram_config WHERE id = 1`)
	var cfg domain.TelegramConfig
	var enabled int
	err := s.db.QueryRowContext(ctx, `
		SELECT id, bot_token, bot_username, is_enabled, updated_at
		FROM telegram_config WHERE id = 1`).Scan(
		&cfg.ID, &cfg.BotToken, &cfg.BotUsername, &enabled, &cfg.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return &domain.TelegramConfig{}, nil
	}
	if err != nil {
		return nil, err
	}
	if s.secrets != nil {
		cfg.BotToken, err = s.secrets.Decrypt(cfg.BotToken)
		if err != nil {
			return nil, err
		}
	}
	cfg.IsEnabled = enabled != 0
	return &cfg, nil
}

func (s *Store) UpsertTelegramConfig(ctx context.Context, cfg domain.TelegramConfig) error {
	token := cfg.BotToken
	if s.secrets != nil {
		var err error
		token, err = s.secrets.Encrypt(token)
		if err != nil {
			return err
		}
	}
	debug.RecordQuery(ctx, `INSERT INTO telegram_config (id, bot_token, bot_username, is_enabled, updated_at) VALUES (1, ?, ?, ?, CURRENT_TIMESTAMP) ON CONFLICT(id) DO UPDATE SET ...`)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO telegram_config (id, bot_token, bot_username, is_enabled, updated_at)
		VALUES (1, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			bot_token=excluded.bot_token,
			bot_username=excluded.bot_username,
			is_enabled=excluded.is_enabled,
			updated_at=CURRENT_TIMESTAMP`,
		token, cfg.BotUsername, boolToInt(cfg.IsEnabled),
	)
	return err
}

func (s *Store) ListTelegramDestinations(ctx context.Context) ([]domain.TelegramDestination, error) {
	return s.listTelegramDestinations(ctx, false, false)
}

func (s *Store) ListActiveTelegramDestinations(ctx context.Context) ([]domain.TelegramDestination, error) {
	return s.listTelegramDestinations(ctx, true, false)
}

func (s *Store) ListActiveAdminTelegramDestinations(ctx context.Context) ([]domain.TelegramDestination, error) {
	return s.listTelegramDestinations(ctx, true, true)
}

func (s *Store) listTelegramDestinations(ctx context.Context, activeOnly, adminOnly bool) ([]domain.TelegramDestination, error) {
	conditions := make([]string, 0, 2)
	if activeOnly {
		conditions = append(conditions, "u.is_active = 1")
	}
	if adminOnly {
		conditions = append(conditions, "u.role = 'admin'")
	}
	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}
	debug.RecordQuery(ctx, `SELECT destination and user fields FROM telegram_destinations JOIN users`+where+` ORDER BY username`)
	rows, err := s.db.QueryContext(ctx, `
		SELECT d.id, d.user_id, u.username, u.is_active, d.chat_id, d.chat_title,
		       d.chat_type, d.created_at, d.updated_at
		FROM telegram_destinations d
		JOIN users u ON u.id = d.user_id`+where+`
		ORDER BY u.username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var destinations []domain.TelegramDestination
	for rows.Next() {
		var destination domain.TelegramDestination
		var active int
		if err := rows.Scan(&destination.ID, &destination.UserID, &destination.Username, &active,
			&destination.ChatID, &destination.ChatTitle, &destination.ChatType,
			&destination.CreatedAt, &destination.UpdatedAt); err != nil {
			return nil, err
		}
		destination.UserIsActive = active != 0
		destinations = append(destinations, destination)
	}
	return destinations, rows.Err()
}

func (s *Store) GetTelegramDestinationForUser(ctx context.Context, userID int64) (*domain.TelegramDestination, error) {
	debug.RecordQuery(ctx, `SELECT destination and user fields FROM telegram_destinations JOIN users WHERE user_id = ?`)
	var destination domain.TelegramDestination
	var active int
	err := s.db.QueryRowContext(ctx, `
		SELECT d.id, d.user_id, u.username, u.is_active, d.chat_id, d.chat_title,
		       d.chat_type, d.created_at, d.updated_at
		FROM telegram_destinations d
		JOIN users u ON u.id = d.user_id
		WHERE d.user_id = ?`, userID).Scan(
		&destination.ID, &destination.UserID, &destination.Username, &active,
		&destination.ChatID, &destination.ChatTitle, &destination.ChatType,
		&destination.CreatedAt, &destination.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	destination.UserIsActive = active != 0
	return &destination, nil
}

func (s *Store) CountTelegramDestinations(ctx context.Context) (int, error) {
	debug.RecordQuery(ctx, `SELECT COUNT(*) FROM telegram_destinations`)
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM telegram_destinations`).Scan(&count)
	return count, err
}

func (s *Store) UpsertTelegramDestination(ctx context.Context, destination domain.TelegramDestination) (int64, error) {
	debug.RecordQuery(ctx, `INSERT INTO telegram_destinations (user_id, chat_id, chat_title, chat_type) VALUES (?, ?, ?, 'private') ON CONFLICT(user_id) DO UPDATE SET ...`)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO telegram_destinations (user_id, chat_id, chat_title, chat_type)
		VALUES (?, ?, ?, 'private')
		ON CONFLICT(user_id) DO UPDATE SET
			chat_id=excluded.chat_id,
			chat_title=excluded.chat_title,
			chat_type='private',
			updated_at=CURRENT_TIMESTAMP`,
		destination.UserID, destination.ChatID, destination.ChatTitle)
	if err != nil {
		return 0, err
	}
	var id int64
	err = s.db.QueryRowContext(ctx, `SELECT id FROM telegram_destinations WHERE user_id = ?`, destination.UserID).Scan(&id)
	return id, err
}

func (s *Store) DeleteTelegramDestinationForUser(ctx context.Context, userID int64) error {
	debug.RecordQuery(ctx, `DELETE FROM telegram_destinations WHERE user_id = ?`)
	_, err := s.db.ExecContext(ctx, `DELETE FROM telegram_destinations WHERE user_id = ?`, userID)
	return err
}

func (s *Store) DeleteAllTelegramDestinations(ctx context.Context) error {
	debug.RecordQuery(ctx, `DELETE FROM telegram_destinations`)
	_, err := s.db.ExecContext(ctx, `DELETE FROM telegram_destinations`)
	return err
}

func (s *Store) DeleteTelegramConfig(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, query := range []string{
		`DELETE FROM telegram_delivery_status`,
		`DELETE FROM telegram_alert_deliveries`,
		`DELETE FROM telegram_destinations`,
		`DELETE FROM telegram_config`,
	} {
		if _, err := tx.ExecContext(ctx, query); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ListCriticalTelegramSentAlertIDs(ctx context.Context, destinationID int64, alertIDs []string) (map[string]bool, error) {
	if len(alertIDs) == 0 {
		return map[string]bool{}, nil
	}
	placeholders, args := stringInArgs(alertIDs)
	args = append([]any{destinationID}, args...)
	debug.RecordQuery(ctx, `SELECT alert_id FROM telegram_alert_deliveries WHERE destination_id = ? AND alert_id IN (...) AND active_sent_at IS NOT NULL`)
	rows, err := s.db.QueryContext(ctx, `SELECT alert_id FROM telegram_alert_deliveries
		WHERE destination_id = ? AND alert_id IN (`+placeholders+`) AND active_sent_at IS NOT NULL`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]bool)
	for rows.Next() {
		var alertID string
		if err := rows.Scan(&alertID); err != nil {
			return nil, err
		}
		result[alertID] = true
	}
	return result, rows.Err()
}

func (s *Store) MarkAlertCriticalTelegramsSent(ctx context.Context, destinationID int64, alertIDs []string, sentAt time.Time) error {
	if len(alertIDs) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, alertID := range alertIDs {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO telegram_alert_deliveries (alert_id, destination_id, active_sent_at, resolution_sent_at)
			VALUES (?, ?, ?, NULL)
			ON CONFLICT(alert_id, destination_id) DO UPDATE SET
				active_sent_at=excluded.active_sent_at,
				resolution_sent_at=NULL`, alertID, destinationID, sentAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ListPendingAlertResolutionTelegrams(ctx context.Context, destinationID int64) ([]domain.Alert, error) {
	debug.RecordQuery(ctx, `SELECT alert state fields FROM alert_states JOIN telegram_alert_deliveries WHERE destination_id = ? AND is_present = 0 AND active_sent_at IS NOT NULL AND resolution_sent_at IS NULL`)
	rows, err := s.db.QueryContext(ctx, `
		SELECT s.alert_id, s.severity, s.title, s.message, s.server_name, s.server_type,
		       s.server_id, s.store_name, s.vmid, s.vm_name
		FROM alert_states s
		JOIN telegram_alert_deliveries d ON d.alert_id = s.alert_id
		WHERE d.destination_id = ? AND s.is_present = 0
		  AND d.active_sent_at IS NOT NULL AND d.resolution_sent_at IS NULL
		ORDER BY s.updated_at DESC`, destinationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var alerts []domain.Alert
	for rows.Next() {
		var alert domain.Alert
		if err := rows.Scan(&alert.ID, &alert.Severity, &alert.Title, &alert.Message, &alert.ServerName,
			&alert.ServerType, &alert.ServerID, &alert.StoreName, &alert.VMID, &alert.VMName); err != nil {
			return nil, err
		}
		alert.Type = strings.Split(alert.ID, ":")[0]
		alerts = append(alerts, alert)
	}
	return alerts, rows.Err()
}

func (s *Store) MarkAlertResolutionTelegramsSent(ctx context.Context, destinationID int64, alertIDs []string, sentAt time.Time) error {
	if len(alertIDs) == 0 {
		return nil
	}
	placeholders, args := stringInArgs(alertIDs)
	args = append([]any{sentAt, destinationID}, args...)
	debug.RecordQuery(ctx, `UPDATE telegram_alert_deliveries SET resolution_sent_at = ? WHERE destination_id = ? AND alert_id IN (...)`)
	_, err := s.db.ExecContext(ctx, `UPDATE telegram_alert_deliveries
		SET resolution_sent_at = ? WHERE destination_id = ? AND alert_id IN (`+placeholders+`)`, args...)
	return err
}

func (s *Store) GetTelegramDeliveryStatus(ctx context.Context) (*domain.TelegramDeliveryStatus, error) {
	debug.RecordQuery(ctx, `SELECT last_attempt_at, last_success_at, last_error FROM telegram_delivery_status WHERE id = 1`)
	var attempt, success sql.NullTime
	var lastError sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT last_attempt_at, last_success_at, last_error
		FROM telegram_delivery_status WHERE id = 1`).Scan(&attempt, &success, &lastError)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	status := &domain.TelegramDeliveryStatus{LastError: lastError.String}
	if attempt.Valid {
		status.LastAttemptAt = &attempt.Time
	}
	if success.Valid {
		status.LastSuccessAt = &success.Time
	}
	return status, nil
}

func (s *Store) RecordTelegramDelivery(ctx context.Context, deliveryErr error) error {
	lastError := ""
	if deliveryErr != nil {
		lastError = strings.TrimSpace(deliveryErr.Error())
		runes := []rune(lastError)
		if len(runes) > 500 {
			lastError = string(runes[:500])
		}
	}
	if lastError == "" {
		debug.RecordQuery(ctx, `INSERT INTO telegram_delivery_status (id, last_attempt_at, last_success_at, last_error) VALUES (1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, '') ON CONFLICT(id) DO UPDATE SET ...`)
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO telegram_delivery_status (id, last_attempt_at, last_success_at, last_error)
			VALUES (1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, '')
			ON CONFLICT(id) DO UPDATE SET
				last_attempt_at=CURRENT_TIMESTAMP,
				last_success_at=CURRENT_TIMESTAMP,
				last_error=''`)
		return err
	}
	debug.RecordQuery(ctx, `INSERT INTO telegram_delivery_status (id, last_attempt_at, last_error) VALUES (1, CURRENT_TIMESTAMP, ?) ON CONFLICT(id) DO UPDATE SET ...`)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO telegram_delivery_status (id, last_attempt_at, last_error)
		VALUES (1, CURRENT_TIMESTAMP, ?)
		ON CONFLICT(id) DO UPDATE SET
			last_attempt_at=CURRENT_TIMESTAMP,
			last_error=excluded.last_error`, lastError)
	return err
}
