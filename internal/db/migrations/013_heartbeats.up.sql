CREATE TABLE IF NOT EXISTS server_heartbeats (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    server_type    TEXT NOT NULL,
    server_id      INTEGER NOT NULL,
    hostname       TEXT NOT NULL,
    ip             TEXT,
    public_ip      TEXT,
    client_version TEXT,
    machine_id     TEXT,
    last_seen_at   DATETIME NOT NULL,
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(server_type, server_id)
);

CREATE INDEX IF NOT EXISTS idx_server_heartbeats_type_seen
    ON server_heartbeats(server_type, last_seen_at);

ALTER TABLE email_config ADD COLUMN alert_pve_heartbeat_minutes INTEGER NOT NULL DEFAULT 15;
