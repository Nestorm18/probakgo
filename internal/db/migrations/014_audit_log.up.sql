CREATE TABLE IF NOT EXISTS audit_log (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    actor_username  TEXT    NOT NULL DEFAULT '',
    actor_role      TEXT    NOT NULL DEFAULT '',
    actor_ip        TEXT    NOT NULL DEFAULT '',
    action          TEXT    NOT NULL,
    target_type     TEXT    NOT NULL DEFAULT '',
    target_id       TEXT    NOT NULL DEFAULT '',
    target_name     TEXT    NOT NULL DEFAULT '',
    metadata        TEXT    NOT NULL DEFAULT '{}',
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_audit_log_created_at ON audit_log(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_log_action ON audit_log(action);
CREATE INDEX IF NOT EXISTS idx_audit_log_target ON audit_log(target_type, target_id);
