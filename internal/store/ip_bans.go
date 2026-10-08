package store

import (
	"context"
	"database/sql"
	"time"

	"probakgo/internal/debug"
	"probakgo/internal/domain"
	"probakgo/internal/ratelimit"
)

func (s *Store) ListIPBans(ctx context.Context) ([]ratelimit.IPBan, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT ip, ban_count, ban_expiry, banned_at FROM ip_bans ORDER BY banned_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ratelimit.IPBan
	for rows.Next() {
		var b ratelimit.IPBan
		var expiry sql.NullString
		var bannedAt string
		if err := rows.Scan(&b.IP, &b.BanCount, &expiry, &bannedAt); err != nil {
			return nil, err
		}
		b.BannedAt, _ = time.Parse(time.RFC3339, bannedAt)
		if expiry.Valid {
			t, _ := time.Parse(time.RFC3339, expiry.String)
			b.BanExpiry = &t
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Store) UpsertIPBan(ctx context.Context, b ratelimit.IPBan) error {
	var expiry any
	if b.BanExpiry != nil {
		expiry = b.BanExpiry.UTC().Format(time.RFC3339)
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO ip_bans (ip, ban_count, ban_expiry, banned_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(ip) DO UPDATE SET
			ban_count  = excluded.ban_count,
			ban_expiry = excluded.ban_expiry,
			banned_at  = excluded.banned_at`,
		b.IP, b.BanCount, expiry, b.BannedAt.UTC().Format(time.RFC3339),
	)
	return err
}

func (s *Store) DeleteIPBan(ctx context.Context, ip string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM ip_bans WHERE ip = ?`, ip)
	return err
}

// Unauthenticated clients choose these values; bound what every attempt stores.
const (
	maxLoginAttemptUsernameLength  = 128
	maxLoginAttemptUserAgentLength = 512
	maxLoginAttemptIPLength        = 64
)

func (s *Store) InsertLoginAttempt(ctx context.Context, username, ip, userAgent, clientDetails, result, reason string) error {
	debug.RecordQuery(ctx, `INSERT INTO login_attempts (username, ip, user_agent, client_details, result, reason) VALUES (?, ?, ?, ?, ?, ?)`)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO login_attempts (username, ip, user_agent, client_details, result, reason)
		VALUES (?, ?, ?, ?, ?, ?)`,
		truncateRunes(username, maxLoginAttemptUsernameLength),
		truncateRunes(ip, maxLoginAttemptIPLength),
		truncateRunes(userAgent, maxLoginAttemptUserAgentLength),
		clientDetails, result, reason,
	)
	return err
}

// DeleteOldLoginAttempts removes attempts older than cutoff. attempted_at is
// written by SQLite as UTC text, so the cutoff uses the same format.
func (s *Store) DeleteOldLoginAttempts(ctx context.Context, cutoff time.Time) (int64, error) {
	debug.RecordQuery(ctx, `DELETE FROM login_attempts WHERE attempted_at < ?`)
	res, err := s.db.ExecContext(ctx, `DELETE FROM login_attempts WHERE attempted_at < ?`,
		sqliteUTC(cutoff))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func truncateRunes(value string, max int) string {
	count := 0
	for i := range value {
		if count == max {
			return value[:i]
		}
		count++
	}
	return value
}

func (s *Store) ListLoginAttempts(ctx context.Context, limit int) ([]domain.LoginAttempt, error) {
	return s.ListLoginAttemptsPage(ctx, limit, 0)
}

func (s *Store) ListLoginAttemptsPage(ctx context.Context, limit, offset int) ([]domain.LoginAttempt, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	debug.RecordQuery(ctx, `SELECT id, username, ip, user_agent, client_details, result, reason, attempted_at FROM login_attempts ORDER BY attempted_at DESC, id DESC LIMIT ? OFFSET ?`)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, username, ip, user_agent, client_details, result, reason, attempted_at
		FROM login_attempts
		ORDER BY attempted_at DESC, id DESC
		LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.LoginAttempt
	for rows.Next() {
		var a domain.LoginAttempt
		if err := rows.Scan(&a.ID, &a.Username, &a.IP, &a.UserAgent, &a.ClientDetails, &a.Result, &a.Reason, &a.AttemptedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
