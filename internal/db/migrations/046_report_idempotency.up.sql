ALTER TABLE pve_reports ADD COLUMN report_id TEXT NOT NULL DEFAULT '';
ALTER TABLE pbs_reports ADD COLUMN report_id TEXT NOT NULL DEFAULT '';
ALTER TABLE windows_reports ADD COLUMN report_id TEXT NOT NULL DEFAULT '';

CREATE UNIQUE INDEX idx_pve_reports_server_report_id
ON pve_reports(server_id, report_id)
WHERE report_id <> '';

CREATE UNIQUE INDEX idx_pbs_reports_server_report_id
ON pbs_reports(server_id, report_id)
WHERE report_id <> '';

CREATE UNIQUE INDEX idx_windows_reports_server_report_id
ON windows_reports(server_id, report_id)
WHERE report_id <> '';
