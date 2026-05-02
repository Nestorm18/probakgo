DELETE FROM api_keys WHERE key_type = 'mobile';

CREATE TABLE api_keys_new (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    key         TEXT    NOT NULL UNIQUE,
    name        TEXT    NOT NULL,
    key_type    TEXT    NOT NULL CHECK(key_type IN ('server','admin')),
    is_active   INTEGER NOT NULL DEFAULT 1,
    machine_id  TEXT,
    last_used   DATETIME,
    server_name TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO api_keys_new SELECT id, key, name, key_type, is_active, machine_id, last_used, server_name, created_at FROM api_keys;

DROP TABLE api_keys;
ALTER TABLE api_keys_new RENAME TO api_keys;

CREATE INDEX IF NOT EXISTS idx_api_keys_type ON api_keys(key_type);
