CREATE TABLE IF NOT EXISTS windows_servers (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    name           TEXT NOT NULL,
    ip             TEXT NOT NULL DEFAULT '',
    public_ip      TEXT NOT NULL DEFAULT '',
    client_version TEXT NOT NULL DEFAULT '',
    machine_id     TEXT NOT NULL DEFAULT '',
    api_key_id     INTEGER UNIQUE REFERENCES api_keys(id) ON DELETE SET NULL,
    is_deleted     INTEGER NOT NULL DEFAULT 0,
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_windows_servers_name ON windows_servers(name);
CREATE INDEX IF NOT EXISTS idx_windows_servers_api_key_id ON windows_servers(api_key_id);

CREATE TABLE IF NOT EXISTS windows_reports (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id   INTEGER NOT NULL REFERENCES windows_servers(id) ON DELETE CASCADE,
    reported_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    is_stale    INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_windows_reports_server_reported_at
ON windows_reports(server_id, reported_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS windows_disks (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    report_id   INTEGER NOT NULL REFERENCES windows_reports(id) ON DELETE CASCADE,
    name        TEXT NOT NULL DEFAULT '',
    label       TEXT NOT NULL DEFAULT '',
    file_system TEXT NOT NULL DEFAULT '',
    drive_type  TEXT NOT NULL DEFAULT '',
    total       INTEGER NOT NULL DEFAULT 0,
    used        INTEGER NOT NULL DEFAULT 0,
    free        INTEGER NOT NULL DEFAULT 0,
    health      TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_windows_disks_report ON windows_disks(report_id);
