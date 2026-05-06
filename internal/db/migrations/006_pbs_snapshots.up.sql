CREATE TABLE IF NOT EXISTS pbs_snapshots (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    store_id           INTEGER NOT NULL REFERENCES pbs_stores(id),
    backup_type        TEXT    NOT NULL DEFAULT '',
    backup_id          TEXT    NOT NULL DEFAULT '',
    last_backup        INTEGER NOT NULL DEFAULT 0,
    backup_count       INTEGER NOT NULL DEFAULT 0,
    owner              TEXT    NOT NULL DEFAULT '',
    comment            TEXT    NOT NULL DEFAULT '',
    verification_state TEXT    NOT NULL DEFAULT '',
    size               INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_pbs_snapshots_store ON pbs_snapshots(store_id);

ALTER TABLE email_config ADD COLUMN alert_pbs_stale_hours INTEGER NOT NULL DEFAULT 48;
