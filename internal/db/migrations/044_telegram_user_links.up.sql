-- Migration 043 existed briefly with two incompatible Telegram destination
-- layouts. Rebuild only the link and per-link delivery tables so databases
-- that already recorded that migration converge on the user-owned model.
-- The global bot configuration and delivery status are intentionally kept.
DROP TABLE IF EXISTS telegram_alert_deliveries;
DROP TABLE IF EXISTS telegram_destinations;

CREATE TABLE telegram_destinations (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    chat_id    TEXT NOT NULL UNIQUE,
    chat_title TEXT NOT NULL DEFAULT '',
    chat_type  TEXT NOT NULL DEFAULT 'private' CHECK (chat_type = 'private'),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_telegram_destinations_user
ON telegram_destinations(user_id);

CREATE TABLE telegram_alert_deliveries (
    alert_id           TEXT NOT NULL REFERENCES alert_states(alert_id) ON DELETE CASCADE,
    destination_id     INTEGER NOT NULL REFERENCES telegram_destinations(id) ON DELETE CASCADE,
    active_sent_at     DATETIME,
    resolution_sent_at DATETIME,
    PRIMARY KEY (alert_id, destination_id)
);

CREATE INDEX idx_telegram_deliveries_destination
ON telegram_alert_deliveries(destination_id, active_sent_at, resolution_sent_at);
