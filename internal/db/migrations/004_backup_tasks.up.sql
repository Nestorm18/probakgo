CREATE TABLE IF NOT EXISTS pve_backup_tasks (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    report_id  INTEGER NOT NULL REFERENCES pve_reports(id),
    vmid       INTEGER NOT NULL,
    vm_name    TEXT    NOT NULL DEFAULT '',
    status     TEXT    NOT NULL DEFAULT '',
    starttime  INTEGER NOT NULL DEFAULT 0,
    endtime    INTEGER NOT NULL DEFAULT 0,
    duration   INTEGER NOT NULL DEFAULT 0,
    size       INTEGER NOT NULL DEFAULT 0,
    filename   TEXT    NOT NULL DEFAULT ''
);
