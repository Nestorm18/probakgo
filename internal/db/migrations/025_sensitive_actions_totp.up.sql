ALTER TABLE email_config ADD COLUMN sensitive_actions_require_totp INTEGER NOT NULL DEFAULT 0;
