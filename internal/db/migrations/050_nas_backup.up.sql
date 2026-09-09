CREATE TABLE nas_backup_config (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    enabled INTEGER NOT NULL DEFAULT 0,
    host TEXT NOT NULL DEFAULT '',
    port INTEGER NOT NULL DEFAULT 22,
    username TEXT NOT NULL DEFAULT '',
    password TEXT NOT NULL DEFAULT '',
    directory TEXT NOT NULL DEFAULT '',
    fingerprint TEXT NOT NULL DEFAULT '',
    send_time TEXT NOT NULL DEFAULT '03:00',
    last_attempt TEXT NOT NULL DEFAULT '',
    last_success TEXT NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT ''
);
INSERT INTO nas_backup_config (id) VALUES (1);
