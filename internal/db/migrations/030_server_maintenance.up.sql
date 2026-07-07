CREATE TABLE IF NOT EXISTS server_maintenance (
    server_type       TEXT    NOT NULL,
    server_id         INTEGER NOT NULL,
    maintenance_until INTEGER NOT NULL,
    reason            TEXT    NOT NULL DEFAULT '',
    created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (server_type, server_id)
);

CREATE INDEX IF NOT EXISTS idx_server_maintenance_until
ON server_maintenance(maintenance_until);
