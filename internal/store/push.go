package store

import (
	"context"
	"database/sql"
	"errors"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"probakgo/internal/debug"
	"probakgo/internal/netutil"
)

// PushSubscription is one browser endpoint registered for a user.
// The (endpoint, p256dh, auth) triple is what the browser's PushManager hands
// us when the user clicks "enable push" on the PWA.
type PushSubscription struct {
	ID         int64
	UserID     int64
	Endpoint   string
	P256DH     string
	Auth       string
	UserAgent  string
	CreatedAt  time.Time
	LastSeenAt time.Time
}

// PushConfig holds the server-wide VAPID key pair used to sign Web Push
// requests. The private key is stored encrypted with the same AES-GCM secretbox
// as SMTP passwords, TOTP secrets and API keys.
type PushConfig struct {
	PublicKey  string
	PrivateKey string
	Subject    string
}

// ErrInvalidPushSubscription identifies malformed or unsafe browser input.
var ErrInvalidPushSubscription = errors.New("invalid push subscription")

const maxPushSubscriptionsPerUser = 10

// GetPushConfig returns the single push_config row. When VAPID keys have not
// been generated yet, every field is the zero value and the caller should
// generate them via EnsurePushConfig before sending notifications.
func (s *Store) GetPushConfig(ctx context.Context) (*PushConfig, error) {
	debug.RecordQuery(ctx, `SELECT public_key, private_key, subject FROM push_config WHERE id = 1`)
	row := s.db.QueryRowContext(ctx, `SELECT public_key, private_key, subject FROM push_config WHERE id = 1`)
	var c PushConfig
	if err := row.Scan(&c.PublicKey, &c.PrivateKey, &c.Subject); err != nil {
		if err == sql.ErrNoRows {
			return &PushConfig{}, nil
		}
		return nil, err
	}
	if s.secrets != nil && c.PrivateKey != "" {
		plain, err := s.secrets.Decrypt(c.PrivateKey)
		if err != nil {
			return nil, err
		}
		c.PrivateKey = plain
	}
	return &c, nil
}

// UpsertPushConfig stores the VAPID key pair. The private key is encrypted at
// rest when a secretbox is configured.
func (s *Store) UpsertPushConfig(ctx context.Context, c PushConfig) error {
	storedPrivate := c.PrivateKey
	if s.secrets != nil && storedPrivate != "" {
		enc, err := s.secrets.Encrypt(storedPrivate)
		if err != nil {
			return err
		}
		storedPrivate = enc
	}
	debug.RecordQuery(ctx, `INSERT INTO push_config (id, public_key, private_key, subject, updated_at) VALUES (1, ?, ?, ?, CURRENT_TIMESTAMP) ON CONFLICT(id) DO UPDATE SET public_key=excluded.public_key, private_key=excluded.private_key, subject=excluded.subject, updated_at=CURRENT_TIMESTAMP`)
	_, err := s.db.ExecContext(ctx, `INSERT INTO push_config (id, public_key, private_key, subject, updated_at)
		VALUES (1, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			public_key=excluded.public_key,
			private_key=excluded.private_key,
			subject=excluded.subject,
			updated_at=CURRENT_TIMESTAMP`,
		c.PublicKey, storedPrivate, c.Subject)
	return err
}

// AddPushSubscription registers (or refreshes) a push subscription for a user.
// If the same endpoint already exists it is reassigned to the requesting user
// and its last_seen_at + user_agent are refreshed.
func (s *Store) AddPushSubscription(ctx context.Context, sub PushSubscription) (int64, error) {
	if err := validatePushSubscription(sub); err != nil {
		return 0, err
	}
	debug.RecordQuery(ctx, `INSERT INTO push_subscriptions (user_id, endpoint, p256dh, auth, user_agent, last_seen_at) VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP) ON CONFLICT(endpoint) DO UPDATE SET user_id=excluded.user_id, p256dh=excluded.p256dh, auth=excluded.auth, user_agent=excluded.user_agent, last_seen_at=CURRENT_TIMESTAMP`)
	_, err := s.db.ExecContext(ctx, `INSERT INTO push_subscriptions
		(user_id, endpoint, p256dh, auth, user_agent, last_seen_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(endpoint) DO UPDATE SET
			user_id=excluded.user_id,
			p256dh=excluded.p256dh,
			auth=excluded.auth,
			user_agent=excluded.user_agent,
			last_seen_at=CURRENT_TIMESTAMP`,
		sub.UserID, sub.Endpoint, sub.P256DH, sub.Auth, sub.UserAgent)
	if err != nil {
		return 0, err
	}
	// Keep a compromised or buggy client from growing the fan-out without
	// bound. The newest endpoints win; normal users usually have one or two.
	if _, err := s.db.ExecContext(ctx, `DELETE FROM push_subscriptions
		WHERE user_id = ? AND id NOT IN (
			SELECT id FROM push_subscriptions WHERE user_id = ?
			ORDER BY last_seen_at DESC, id DESC LIMIT ?
		)`, sub.UserID, sub.UserID, maxPushSubscriptionsPerUser); err != nil {
		return 0, err
	}
	var id int64
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM push_subscriptions WHERE endpoint = ? AND user_id = ?`, sub.Endpoint, sub.UserID).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func validatePushSubscription(sub PushSubscription) error {
	if len(sub.Endpoint) == 0 || len(sub.Endpoint) > 4096 {
		return ErrInvalidPushSubscription
	}
	u, err := url.Parse(sub.Endpoint)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" || (u.Port() != "" && u.Port() != "443") {
		return ErrInvalidPushSubscription
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return ErrInvalidPushSubscription
	}
	if ip, err := netip.ParseAddr(host); err == nil && !netutil.IsPublicIP(ip) {
		return ErrInvalidPushSubscription
	}
	if len(sub.P256DH) == 0 || len(sub.P256DH) > 512 || len(sub.Auth) == 0 || len(sub.Auth) > 512 {
		return ErrInvalidPushSubscription
	}
	if len(sub.UserAgent) > 512 || strings.ContainsAny(sub.UserAgent, "\r\n") {
		return ErrInvalidPushSubscription
	}
	return nil
}

// RemovePushSubscription deletes a subscription by endpoint, but only if it
// belongs to the supplied user. This stops a logged-out attacker (or another
// user on the same device) from removing someone else's subscription.
func (s *Store) RemovePushSubscription(ctx context.Context, userID int64, endpoint string) error {
	debug.RecordQuery(ctx, `DELETE FROM push_subscriptions WHERE endpoint = ? AND user_id = ?`)
	_, err := s.db.ExecContext(ctx, `DELETE FROM push_subscriptions WHERE endpoint = ? AND user_id = ?`, endpoint, userID)
	return err
}

// ListPushSubscriptionsByUser returns every subscription that belongs to one
// user. Used by the profile page to show "where am I subscribed?".
func (s *Store) ListPushSubscriptionsByUser(ctx context.Context, userID int64) ([]PushSubscription, error) {
	debug.RecordQuery(ctx, `SELECT id, user_id, endpoint, p256dh, auth, user_agent, created_at, last_seen_at FROM push_subscriptions WHERE user_id = ? ORDER BY last_seen_at DESC`)
	rows, err := s.db.QueryContext(ctx, `SELECT id, user_id, endpoint, p256dh, auth, user_agent, created_at, last_seen_at
		FROM push_subscriptions WHERE user_id = ? ORDER BY last_seen_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPushSubscriptions(rows)
}

// ListAllPushSubscriptions returns every subscription in the database. The
// push sender uses this to fan an alert out to every device that opted in.
func (s *Store) ListAllPushSubscriptions(ctx context.Context) ([]PushSubscription, error) {
	debug.RecordQuery(ctx, `SELECT ps.id, ps.user_id, ps.endpoint, ps.p256dh, ps.auth, ps.user_agent, ps.created_at, ps.last_seen_at FROM push_subscriptions ps JOIN users u ON u.id = ps.user_id WHERE u.is_active = 1 ORDER BY ps.id`)
	rows, err := s.db.QueryContext(ctx, `SELECT ps.id, ps.user_id, ps.endpoint, ps.p256dh, ps.auth, ps.user_agent, ps.created_at, ps.last_seen_at
		FROM push_subscriptions ps
		JOIN users u ON u.id = ps.user_id
		WHERE u.is_active = 1
		ORDER BY ps.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPushSubscriptions(rows)
}

// DeletePushSubscriptionsByUser is the cascade for user deletion. The
// ON DELETE CASCADE on push_subscriptions already covers this, but exposing it
// lets us call it explicitly when a user is disabled.
func (s *Store) DeletePushSubscriptionsByUser(ctx context.Context, userID int64) error {
	debug.RecordQuery(ctx, `DELETE FROM push_subscriptions WHERE user_id = ?`)
	_, err := s.db.ExecContext(ctx, `DELETE FROM push_subscriptions WHERE user_id = ?`, userID)
	return err
}

// CountPushSubscriptions returns the total number of registered subscriptions.
// Used by the profile/settings pages and by tests.
func (s *Store) CountPushSubscriptions(ctx context.Context) (int, error) {
	debug.RecordQuery(ctx, `SELECT COUNT(*) FROM push_subscriptions`)
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM push_subscriptions`).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func scanPushSubscriptions(rows *sql.Rows) ([]PushSubscription, error) {
	var subs []PushSubscription
	for rows.Next() {
		var s PushSubscription
		if err := rows.Scan(&s.ID, &s.UserID, &s.Endpoint, &s.P256DH, &s.Auth, &s.UserAgent, &s.CreatedAt, &s.LastSeenAt); err != nil {
			return nil, err
		}
		subs = append(subs, s)
	}
	return subs, rows.Err()
}
