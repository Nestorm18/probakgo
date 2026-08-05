-- PWA push notification support: per-user Web Push subscriptions and VAPID keys.
-- endpoint is the URL the browser hands to push.service (e.g. FCM/mozilla/autopush).
-- p256dh and auth are the per-subscription ECDH keys the browser generates.

CREATE TABLE IF NOT EXISTS push_subscriptions (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id       INTEGER NOT NULL,
    endpoint      TEXT    NOT NULL,
    p256dh        TEXT    NOT NULL DEFAULT '',
    auth          TEXT    NOT NULL DEFAULT '',
    user_agent    TEXT    NOT NULL DEFAULT '',
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_push_subscriptions_endpoint
    ON push_subscriptions(endpoint);

CREATE INDEX IF NOT EXISTS idx_push_subscriptions_user
    ON push_subscriptions(user_id);

-- Single-row table holding the VAPID key pair used to sign Web Push messages.
-- public_key is stored as plain text (VAPID public keys are not secret).
-- private_key is encrypted at rest with the same AES-GCM secretbox as SMTP/TOTP/API keys.
CREATE TABLE IF NOT EXISTS push_config (
    id           INTEGER PRIMARY KEY CHECK (id = 1),
    public_key   TEXT    NOT NULL DEFAULT '',
    private_key  TEXT    NOT NULL DEFAULT '',
    subject      TEXT    NOT NULL DEFAULT 'mailto:admin@example.com',
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT OR IGNORE INTO push_config (id, public_key, private_key, subject)
    VALUES (1, '', '', 'mailto:admin@example.com');
