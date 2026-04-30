CREATE TABLE IF NOT EXISTS ip_bans (
    ip         TEXT    PRIMARY KEY,
    ban_count  INTEGER NOT NULL DEFAULT 1,
    ban_expiry TEXT,   -- NULL = permanent
    banned_at  TEXT    NOT NULL
);
