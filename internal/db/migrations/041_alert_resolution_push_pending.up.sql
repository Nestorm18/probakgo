-- Track resolution delivery independently for the same reason as active
-- critical push notifications.
ALTER TABLE alert_states ADD COLUMN resolution_push_pending INTEGER NOT NULL DEFAULT 0;
