ALTER TABLE email_config ADD COLUMN enforce_totp_non_readers INTEGER NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN totp_grace_started_at DATETIME;
