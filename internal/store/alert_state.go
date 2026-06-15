package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"probakgo/internal/debug"
	"probakgo/internal/domain"
)

type alertStateRow struct {
	AlertID    string
	IsPresent  bool
	Severity   string
	Title      string
	Message    string
	ServerName string
	ServerType string
	ServerID   int64
	StoreName  string
	VMID       int64
	VMName     string
}

func (s *Store) SyncAlertStates(ctx context.Context, alerts []domain.Alert) error {
	debug.RecordQuery(ctx, `SELECT alert_id, is_present, severity, title, message, server_name, server_type, server_id, store_name, vmid, vm_name FROM alert_states`)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `
		SELECT alert_id, is_present, severity, title, message, server_name, server_type, server_id, store_name, vmid, vm_name
		FROM alert_states`)
	if err != nil {
		return err
	}
	existing := make(map[string]alertStateRow)
	for rows.Next() {
		var row alertStateRow
		var isPresent int
		if err := rows.Scan(&row.AlertID, &isPresent, &row.Severity, &row.Title, &row.Message,
			&row.ServerName, &row.ServerType, &row.ServerID, &row.StoreName, &row.VMID, &row.VMName); err != nil {
			rows.Close()
			return err
		}
		row.IsPresent = isPresent != 0
		existing[row.AlertID] = row
	}
	if err := rows.Close(); err != nil {
		return err
	}

	now := time.Now()
	current := make(map[string]domain.Alert, len(alerts))
	for _, alert := range alerts {
		current[alert.ID] = alert
		state, ok := existing[alert.ID]
		if !ok {
			if err := insertAlertState(ctx, tx, alert, now); err != nil {
				return err
			}
			if err := insertAlertEvent(ctx, tx, alertEventFromAlert(alert, "appeared", "")); err != nil {
				return err
			}
			continue
		}
		if !state.IsPresent {
			if err := insertAlertEvent(ctx, tx, alertEventFromAlert(alert, "appeared", "")); err != nil {
				return err
			}
		}
		if err := updateAlertState(ctx, tx, alert, now); err != nil {
			return err
		}
	}

	for alertID, state := range existing {
		if !state.IsPresent {
			continue
		}
		if _, ok := current[alertID]; ok {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE alert_states
			SET is_present = 0, updated_at = ?
			WHERE alert_id = ?`, now, alertID); err != nil {
			return err
		}
		if err := insertAlertEvent(ctx, tx, alertEventFromState(state, "resolved", "")); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) InsertAlertStateEvent(ctx context.Context, entry domain.AlertStateEvent) error {
	debug.RecordQuery(ctx, `INSERT INTO alert_state_events (...) VALUES (...)`)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO alert_state_events (
			alert_id, event_type, severity, title, message, server_name, server_type, server_id,
			store_name, vmid, vm_name, note
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.AlertID, entry.EventType, entry.Severity, entry.Title, entry.Message,
		entry.ServerName, entry.ServerType, entry.ServerID, entry.StoreName, entry.VMID, entry.VMName, entry.Note,
	)
	return err
}

func (s *Store) ListAlertStateEvents(ctx context.Context, limit int) ([]domain.AlertStateEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	debug.RecordQuery(ctx, `SELECT id, alert_id, event_type, severity, title, message, server_name, server_type, server_id, store_name, vmid, vm_name, note, created_at FROM alert_state_events ORDER BY created_at DESC LIMIT ?`)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, alert_id, event_type, severity, title, message, server_name, server_type, server_id,
		       store_name, vmid, vm_name, note, created_at
		FROM alert_state_events
		ORDER BY created_at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []domain.AlertStateEvent
	for rows.Next() {
		var ev domain.AlertStateEvent
		if err := rows.Scan(&ev.ID, &ev.AlertID, &ev.EventType, &ev.Severity, &ev.Title, &ev.Message,
			&ev.ServerName, &ev.ServerType, &ev.ServerID, &ev.StoreName, &ev.VMID, &ev.VMName, &ev.Note, &ev.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, ev)
	}
	return events, rows.Err()
}

func (s *Store) GetAlertStateEventInfo(ctx context.Context, alertID string) (domain.AlertStateEvent, error) {
	debug.RecordQuery(ctx, `SELECT alert_id, severity, title, message, server_name, server_type, server_id, store_name, vmid, vm_name FROM alert_states WHERE alert_id = ?`)
	row := s.db.QueryRowContext(ctx, `
		SELECT alert_id, severity, title, message, server_name, server_type, server_id, store_name, vmid, vm_name
		FROM alert_states
		WHERE alert_id = ?`, alertID)
	var ev domain.AlertStateEvent
	err := row.Scan(&ev.AlertID, &ev.Severity, &ev.Title, &ev.Message, &ev.ServerName, &ev.ServerType, &ev.ServerID, &ev.StoreName, &ev.VMID, &ev.VMName)
	if err != nil {
		return domain.AlertStateEvent{}, err
	}
	return ev, nil
}

func insertAlertState(ctx context.Context, tx *sql.Tx, alert domain.Alert, now time.Time) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO alert_states (
			alert_id, is_present, severity, title, message, server_name, server_type, server_id,
			store_name, vmid, vm_name, first_seen_at, last_seen_at, updated_at
		) VALUES (?, 1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		alert.ID, alert.Severity, alert.Title, alert.Message, alert.ServerName, alert.ServerType,
		alert.ServerID, alert.StoreName, alert.VMID, alert.VMName, now, now, now,
	)
	return err
}

func updateAlertState(ctx context.Context, tx *sql.Tx, alert domain.Alert, now time.Time) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE alert_states
		SET is_present = 1, severity = ?, title = ?, message = ?, server_name = ?, server_type = ?,
		    server_id = ?, store_name = ?, vmid = ?, vm_name = ?, last_seen_at = ?, updated_at = ?
		WHERE alert_id = ?`,
		alert.Severity, alert.Title, alert.Message, alert.ServerName, alert.ServerType,
		alert.ServerID, alert.StoreName, alert.VMID, alert.VMName, now, now, alert.ID,
	)
	return err
}

func insertAlertEvent(ctx context.Context, tx *sql.Tx, entry domain.AlertStateEvent) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO alert_state_events (
			alert_id, event_type, severity, title, message, server_name, server_type, server_id,
			store_name, vmid, vm_name, note
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.AlertID, entry.EventType, entry.Severity, entry.Title, entry.Message,
		entry.ServerName, entry.ServerType, entry.ServerID, entry.StoreName, entry.VMID, entry.VMName, entry.Note,
	)
	if err != nil {
		return fmt.Errorf("insert alert event: %w", err)
	}
	return nil
}

func alertEventFromAlert(alert domain.Alert, eventType, note string) domain.AlertStateEvent {
	return domain.AlertStateEvent{
		AlertID: alert.ID, EventType: eventType, Severity: alert.Severity, Title: alert.Title,
		Message: alert.Message, ServerName: alert.ServerName, ServerType: alert.ServerType,
		ServerID: alert.ServerID, StoreName: alert.StoreName, VMID: alert.VMID, VMName: alert.VMName, Note: note,
	}
}

func alertEventFromState(state alertStateRow, eventType, note string) domain.AlertStateEvent {
	return domain.AlertStateEvent{
		AlertID: state.AlertID, EventType: eventType, Severity: state.Severity, Title: state.Title,
		Message: state.Message, ServerName: state.ServerName, ServerType: state.ServerType,
		ServerID: state.ServerID, StoreName: state.StoreName, VMID: state.VMID, VMName: state.VMName, Note: note,
	}
}
