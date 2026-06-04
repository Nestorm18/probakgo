CREATE INDEX IF NOT EXISTS idx_pve_reports_server_reported_at
ON pve_reports(server_id, reported_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_pbs_reports_server_reported_at
ON pbs_reports(server_id, reported_at DESC, id DESC);
