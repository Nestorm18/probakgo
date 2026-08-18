package store

import (
	"context"
	"testing"
)

func TestResetAllDataClearsOperationalDataAndPreservesSecurityTrail(t *testing.T) {
	st := openTestDB(t)
	ctx := context.Background()

	mustExec := func(query string, args ...any) int64 {
		t.Helper()
		res, err := st.db.ExecContext(ctx, query, args...)
		if err != nil {
			t.Fatalf("exec %q: %v", query, err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			t.Fatalf("last insert id for %q: %v", query, err)
		}
		return id
	}

	adminID := mustExec(`INSERT INTO users (username, password_hash, role) VALUES ('admin', 'hash', 'admin')`)
	mustExec(`INSERT INTO audit_log (actor_username, action) VALUES ('admin', 'before_reset')`)

	pbsServerID := mustExec(`INSERT INTO pbs_servers (name) VALUES ('pbs-reset')`)
	pbsReportID := mustExec(`INSERT INTO pbs_reports (server_id) VALUES (?)`, pbsServerID)
	mustExec(`INSERT INTO pbs_maintenance_tasks (report_id, task_type) VALUES (?, 'syncjob')`, pbsReportID)

	windowsServerID := mustExec(`INSERT INTO windows_servers (name) VALUES ('windows-reset')`)
	windowsReportID := mustExec(`INSERT INTO windows_reports (server_id) VALUES (?)`, windowsServerID)
	mustExec(`INSERT INTO windows_disks (report_id, name) VALUES (?, 'C:')`, windowsReportID)

	mustExec(`INSERT INTO email_delivery_status (id, last_error) VALUES (1, 'test')`)
	mustExec(`INSERT INTO telegram_config (id, bot_token) VALUES (1, 'token')`)
	mustExec(`INSERT INTO telegram_destinations (user_id, chat_id, chat_title) VALUES (?, '123', 'Nestor')`, adminID)
	mustExec(`INSERT INTO telegram_delivery_status (id, last_error) VALUES (1, 'test')`)

	if err := st.ResetAllData(ctx); err != nil {
		t.Fatalf("ResetAllData: %v", err)
	}

	for _, table := range []string{
		"pbs_maintenance_tasks",
		"pbs_reports",
		"pbs_servers",
		"windows_disks",
		"windows_reports",
		"windows_servers",
		"email_delivery_status",
		"telegram_delivery_status",
		"telegram_alert_deliveries",
		"telegram_destinations",
		"telegram_config",
	} {
		var count int
		if err := st.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 0 {
			t.Errorf("%s count: got %d, want 0", table, count)
		}
	}

	for _, table := range []string{"users", "audit_log", "schema_migrations"} {
		var count int
		if err := st.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count == 0 {
			t.Errorf("%s should be preserved", table)
		}
	}
}
