-- Push delivery has its own state. Reusing last_critical_email_at would
-- suppress a later email when SMTP is disabled or resend push whenever SMTP
-- has a transient failure.
ALTER TABLE alert_states ADD COLUMN last_critical_push_at DATETIME;
