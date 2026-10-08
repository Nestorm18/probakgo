package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"probakgo/internal/debug"
)

// RevokeSession lists a session ID as closed until expiresAt, when the cookie
// carrying it expires anyway. Expired entries are purged on each call.
func (s *Store) RevokeSession(ctx context.Context, sid string, expiresAt time.Time) error {
	debug.RecordQuery(ctx, `DELETE FROM revoked_sessions WHERE expires_at < ?`)
	if _, err := s.db.ExecContext(ctx, `DELETE FROM revoked_sessions WHERE expires_at < ?`, time.Now().Unix()); err != nil {
		return err
	}
	debug.RecordQuery(ctx, `INSERT INTO revoked_sessions (sid_hash, expires_at) VALUES (?, ?) ON CONFLICT(sid_hash) DO UPDATE SET expires_at=excluded.expires_at`)
	_, err := s.db.ExecContext(ctx, `INSERT INTO revoked_sessions (sid_hash, expires_at) VALUES (?, ?)
		ON CONFLICT(sid_hash) DO UPDATE SET expires_at=excluded.expires_at`, sessionIDHash(sid), expiresAt.Unix())
	return err
}

// IsSessionRevoked reports whether sid was closed by a logout.
func (s *Store) IsSessionRevoked(ctx context.Context, sid string) (bool, error) {
	debug.RecordQuery(ctx, `SELECT EXISTS(SELECT 1 FROM revoked_sessions WHERE sid_hash=?)`)
	var revoked bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM revoked_sessions WHERE sid_hash=?)`, sessionIDHash(sid)).Scan(&revoked)
	return revoked, err
}

// sessionIDHash keeps raw session IDs out of the database.
func sessionIDHash(sid string) string {
	sum := sha256.Sum256([]byte(sid))
	return hex.EncodeToString(sum[:])
}
