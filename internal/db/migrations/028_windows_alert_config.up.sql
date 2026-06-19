ALTER TABLE email_config ADD COLUMN alert_windows_disk_pct INTEGER NOT NULL DEFAULT 90;

CREATE TABLE IF NOT EXISTS windows_alert_config (
    server_id  INTEGER PRIMARY KEY REFERENCES windows_servers(id) ON DELETE CASCADE,
    disk_pct   INTEGER,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
