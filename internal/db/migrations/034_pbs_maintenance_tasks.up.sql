CREATE TABLE IF NOT EXISTS pbs_maintenance_tasks (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    report_id    INTEGER NOT NULL REFERENCES pbs_reports(id),
    task_type    TEXT    NOT NULL,
    job_id       TEXT    NOT NULL DEFAULT '',
    remote       TEXT    NOT NULL DEFAULT '',
    remote_store TEXT    NOT NULL DEFAULT '',
    store        TEXT    NOT NULL DEFAULT '',
    status       TEXT    NOT NULL DEFAULT '',
    start_time   INTEGER NOT NULL DEFAULT 0,
    end_time     INTEGER NOT NULL DEFAULT 0,
    upid         TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_pbs_maintenance_tasks_report ON pbs_maintenance_tasks(report_id);
