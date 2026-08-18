CREATE TABLE IF NOT EXISTS telegram_config (
    id           INTEGER PRIMARY KEY CHECK (id = 1),
    bot_token    TEXT NOT NULL DEFAULT '',
    bot_username TEXT NOT NULL DEFAULT '',
    is_enabled   INTEGER NOT NULL DEFAULT 0,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS telegram_destinations (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    chat_id    TEXT NOT NULL UNIQUE,
    chat_title TEXT NOT NULL DEFAULT '',
    chat_type  TEXT NOT NULL DEFAULT 'private' CHECK (chat_type = 'private'),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_telegram_destinations_user
ON telegram_destinations(user_id);

CREATE TABLE IF NOT EXISTS telegram_delivery_status (
    id              INTEGER PRIMARY KEY CHECK (id = 1),
    last_attempt_at DATETIME,
    last_success_at DATETIME,
    last_error      TEXT NOT NULL DEFAULT ''
);

-- Delivery state is per user destination. Inactive users are excluded by the
-- sender and deleting a user cascades through the destination and this state.
CREATE TABLE IF NOT EXISTS telegram_alert_deliveries (
    alert_id           TEXT NOT NULL REFERENCES alert_states(alert_id) ON DELETE CASCADE,
    destination_id     INTEGER NOT NULL REFERENCES telegram_destinations(id) ON DELETE CASCADE,
    active_sent_at     DATETIME,
    resolution_sent_at DATETIME,
    PRIMARY KEY (alert_id, destination_id)
);

CREATE INDEX IF NOT EXISTS idx_telegram_deliveries_destination
ON telegram_alert_deliveries(destination_id, active_sent_at, resolution_sent_at);
