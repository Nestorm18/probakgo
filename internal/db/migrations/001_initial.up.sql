CREATE TABLE IF NOT EXISTS pve_servers (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT    NOT NULL UNIQUE,
    ip         TEXT,
    public_ip  TEXT,
    client_version TEXT,
    machine_id TEXT,
    is_deleted INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS pve_reports (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id  INTEGER NOT NULL REFERENCES pve_servers(id),
    reported_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    is_stale   INTEGER NOT NULL DEFAULT 0,
    stale_reason TEXT,
    backup_status    TEXT,
    backup_starttime INTEGER,
    backup_endtime   INTEGER,
    backup_duration  INTEGER
);

CREATE TABLE IF NOT EXISTS pve_storages (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    report_id INTEGER NOT NULL REFERENCES pve_reports(id),
    storage   TEXT,
    path      TEXT,
    content   TEXT,
    type      TEXT,
    status    TEXT,
    shared    INTEGER,
    server    TEXT,
    digest    TEXT,
    prune_backups TEXT
);

CREATE TABLE IF NOT EXISTS pve_storage_info (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    storage_id INTEGER NOT NULL REFERENCES pve_storages(id),
    total      INTEGER,
    used       INTEGER,
    avail      INTEGER,
    used_percent REAL,
    active     INTEGER,
    enabled    INTEGER,
    lvl        INTEGER
);

CREATE TABLE IF NOT EXISTS pve_storage_content (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    storage_id INTEGER NOT NULL REFERENCES pve_storages(id),
    vmid       INTEGER,
    format     TEXT,
    size       INTEGER,
    content    TEXT,
    volid      TEXT,
    ctime      INTEGER,
    subtype    TEXT,
    notes      TEXT
);

CREATE TABLE IF NOT EXISTS pbs_servers (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT    NOT NULL UNIQUE,
    ip         TEXT,
    public_ip  TEXT,
    client_version TEXT,
    machine_id TEXT,
    is_deleted INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS pbs_reports (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id   INTEGER NOT NULL REFERENCES pbs_servers(id),
    reported_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    is_stale    INTEGER NOT NULL DEFAULT 0,
    stale_reason TEXT
);

CREATE TABLE IF NOT EXISTS pbs_stores (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    report_id           INTEGER NOT NULL REFERENCES pbs_reports(id),
    store               TEXT,
    total               INTEGER,
    used                INTEGER,
    avail               INTEGER,
    estimated_full_date INTEGER,
    mount_status        TEXT,
    history_start       INTEGER,
    history_delta       INTEGER
);

CREATE TABLE IF NOT EXISTS pbs_store_history (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    store_id INTEGER NOT NULL REFERENCES pbs_stores(id),
    position INTEGER NOT NULL,
    value    REAL
);

CREATE TABLE IF NOT EXISTS pbs_gc_status (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    store_id         INTEGER NOT NULL REFERENCES pbs_stores(id),
    disk_bytes       INTEGER,
    disk_chunks      INTEGER,
    index_data_bytes INTEGER,
    index_file_count INTEGER,
    pending_bytes    INTEGER,
    pending_chunks   INTEGER,
    removed_bad      INTEGER,
    removed_bytes    INTEGER,
    removed_chunks   INTEGER,
    still_bad        INTEGER,
    upid             TEXT
);

CREATE TABLE IF NOT EXISTS api_keys (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    key         TEXT    NOT NULL UNIQUE,
    name        TEXT    NOT NULL,
    key_type    TEXT    NOT NULL CHECK(key_type IN ('server','mobile','admin')),
    is_active   INTEGER NOT NULL DEFAULT 1,
    machine_id  TEXT,
    last_used   DATETIME,
    server_name TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    username      TEXT    NOT NULL UNIQUE,
    password_hash TEXT    NOT NULL,
    role          TEXT    NOT NULL DEFAULT 'reader' CHECK(role IN ('admin','editor','reader')),
    is_active     INTEGER NOT NULL DEFAULT 1,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS vm_backup_configs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    server_name TEXT    NOT NULL,
    vm_id       TEXT    NOT NULL,
    vm_name     TEXT,
    monday      INTEGER NOT NULL DEFAULT 0,
    tuesday     INTEGER NOT NULL DEFAULT 0,
    wednesday   INTEGER NOT NULL DEFAULT 0,
    thursday    INTEGER NOT NULL DEFAULT 0,
    friday      INTEGER NOT NULL DEFAULT 0,
    saturday    INTEGER NOT NULL DEFAULT 0,
    sunday      INTEGER NOT NULL DEFAULT 0,
    is_excluded INTEGER NOT NULL DEFAULT 0,
    is_deleted  INTEGER NOT NULL DEFAULT 0,
    deleted_at  DATETIME,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(server_name, vm_id)
);

CREATE TABLE IF NOT EXISTS email_config (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    smtp_host  TEXT,
    smtp_port  INTEGER NOT NULL DEFAULT 587,
    smtp_user  TEXT,
    smtp_password TEXT,
    recipients TEXT,
    is_enabled INTEGER NOT NULL DEFAULT 0,
    send_time  TEXT    NOT NULL DEFAULT '08:00'
);

CREATE INDEX IF NOT EXISTS idx_pve_reports_server ON pve_reports(server_id);
CREATE INDEX IF NOT EXISTS idx_pbs_reports_server ON pbs_reports(server_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_key       ON api_keys(key);
CREATE INDEX IF NOT EXISTS idx_api_keys_type      ON api_keys(key_type);
