CREATE TABLE IF NOT EXISTS email_delivery_status (
    id              INTEGER PRIMARY KEY CHECK (id = 1),
    last_attempt_at DATETIME,
    last_success_at DATETIME,
    last_error      TEXT NOT NULL DEFAULT ''
);
