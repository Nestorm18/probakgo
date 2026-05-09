package store

import (
	"context"
	"database/sql"
	"time"

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
