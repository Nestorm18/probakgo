ALTER TABLE email_config ADD COLUMN alert_email_batch_minutes INTEGER NOT NULL DEFAULT 0 CHECK (alert_email_batch_minutes IN (0, 5, 15));
CREATE TABLE alert_email_batch (id INTEGER PRIMARY KEY CHECK (id = 1), pending_since INTEGER NOT NULL);
