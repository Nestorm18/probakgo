package store

import (
	"context"

	"probakgo/internal/debug"
	"probakgo/internal/domain"
)

func (s *Store) InsertAuditLog(ctx context.Context, entry domain.AuditLog) error {
	debug.RecordQuery(ctx, `INSERT INTO audit_log (actor_username, actor_role, actor_ip, action, target_type, target_id, target_name, metadata) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO audit_log (actor_username, actor_role, actor_ip, action, target_type, target_id, target_name, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.ActorUsername, entry.ActorRole, entry.ActorIP, entry.Action,
		entry.TargetType, entry.TargetID, entry.TargetName, entry.Metadata,
	)
	return err
}

func (s *Store) ListAuditLogs(ctx context.Context, limit int) ([]domain.AuditLog, error) {
	if limit <= 0 {
		limit = 200
	}
	debug.RecordQuery(ctx, `SELECT id, actor_username, actor_role, actor_ip, action, target_type, target_id, target_name, metadata, created_at FROM audit_log ORDER BY created_at DESC, id DESC LIMIT ?`)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, actor_username, actor_role, actor_ip, action, target_type, target_id, target_name, metadata, created_at
		FROM audit_log
		ORDER BY created_at DESC, id DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.AuditLog
	for rows.Next() {
		var a domain.AuditLog
		if err := rows.Scan(&a.ID, &a.ActorUsername, &a.ActorRole, &a.ActorIP, &a.Action,
			&a.TargetType, &a.TargetID, &a.TargetName, &a.Metadata, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
