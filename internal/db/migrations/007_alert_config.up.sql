-- Per-PVE-server alert thresholds. NULL = inherit from global email_config value.
CREATE TABLE IF NOT EXISTS pve_alert_config (
    server_id   INTEGER PRIMARY KEY REFERENCES pve_servers(id),
    disk_pct    INTEGER,
    stale_hours INTEGER,
    backup_err  INTEGER,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Per-VM overrides within a PVE server. NULL = inherit from server-level config.
CREATE TABLE IF NOT EXISTS pve_vm_alert_config (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id   INTEGER NOT NULL REFERENCES pve_servers(id),
    vmid        INTEGER NOT NULL,
    backup_err  INTEGER,
    min_size_mb INTEGER,
    UNIQUE(server_id, vmid)
);

-- Per-PBS-server alert thresholds. NULL = inherit from global email_config value.
CREATE TABLE IF NOT EXISTS pbs_alert_config (
    server_id       INTEGER PRIMARY KEY REFERENCES pbs_servers(id),
    disk_pct        INTEGER,
    days_until_full INTEGER,
    stale_hours     INTEGER,
    verify_alert    INTEGER NOT NULL DEFAULT 1,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
