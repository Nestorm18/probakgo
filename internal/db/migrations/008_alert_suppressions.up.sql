CREATE TABLE IF NOT EXISTS alert_suppressions (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    alert_id        TEXT    NOT NULL UNIQUE,
    suppressed_until INTEGER NOT NULL,  -- Unix timestamp
    reason          TEXT    NOT NULL DEFAULT '',
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
