ALTER TABLE email_config ADD COLUMN alert_pve_expected_finish_time TEXT NOT NULL DEFAULT '';

UPDATE email_config
SET alert_pve_expected_finish_time = strftime('%H:%M', send_time, '-5 minutes')
WHERE alert_pve_expected_finish_time = '';
