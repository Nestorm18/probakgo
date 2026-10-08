-- Sessions closed by logout. Cookies are stateless, so a copied cookie stays
-- valid until its own expiry unless its session ID is listed here.
CREATE TABLE IF NOT EXISTS revoked_sessions (
    sid_hash   TEXT PRIMARY KEY,
    expires_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_revoked_sessions_expires_at ON revoked_sessions(expires_at);
