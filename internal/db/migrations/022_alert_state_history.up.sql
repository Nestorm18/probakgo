CREATE TABLE IF NOT EXISTS alert_states (
    alert_id TEXT PRIMARY KEY,
    is_present INTEGER NOT NULL DEFAULT 1,
    severity TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    message TEXT NOT NULL DEFAULT '',
    server_name TEXT NOT NULL DEFAULT '',
    server_type TEXT NOT NULL DEFAULT '',
    server_id INTEGER NOT NULL DEFAULT 0,
    store_name TEXT NOT NULL DEFAULT '',
    vmid INTEGER NOT NULL DEFAULT 0,
    vm_name TEXT NOT NULL DEFAULT '',
    first_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS alert_state_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    alert_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    severity TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    message TEXT NOT NULL DEFAULT '',
    server_name TEXT NOT NULL DEFAULT '',
    server_type TEXT NOT NULL DEFAULT '',
    server_id INTEGER NOT NULL DEFAULT 0,
    store_name TEXT NOT NULL DEFAULT '',
    vmid INTEGER NOT NULL DEFAULT 0,
    vm_name TEXT NOT NULL DEFAULT '',
    note TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_alert_state_events_created
ON alert_state_events(created_at DESC);

CREATE INDEX IF NOT EXISTS idx_alert_state_events_alert
ON alert_state_events(alert_id, created_at DESC);
