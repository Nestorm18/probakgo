package store

import (
	"context"
	"strconv"
	"testing"
	"time"

	"probakgo/internal/domain"
)

func TestHardDeleteServerDataForAPIKey_DeletesOnlyBoundServerAlerts(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)

	key1, err := st.CreateAPIKey(ctx, "nicolas", "duplicate-host", "")
	if err != nil {
		t.Fatalf("create key1: %v", err)
	}
	key2, err := st.CreateAPIKey(ctx, "nicolas-gestion", "duplicate-host", "")
	if err != nil {
		t.Fatalf("create key2: %v", err)
	}

	server1, err := st.UpsertPVEServerForAPIKey(ctx, key1.ID, "duplicate-host", "10.0.0.1", "", "0.0.89", "machine-1")
	if err != nil {
		t.Fatalf("upsert server1: %v", err)
	}
	server2, err := st.UpsertPVEServerForAPIKey(ctx, key2.ID, "duplicate-host", "10.0.0.2", "", "0.0.89", "machine-2")
	if err != nil {
		t.Fatalf("upsert server2: %v", err)
	}

	if err := st.UpsertPVEAlertConfig(ctx, domain.PVEAlertConfig{ServerID: server1, BackupErr: intPtr(1)}); err != nil {
		t.Fatalf("upsert alert config server1: %v", err)
	}
	if err := st.UpsertPVEAlertConfig(ctx, domain.PVEAlertConfig{ServerID: server2, BackupErr: intPtr(1)}); err != nil {
		t.Fatalf("upsert alert config server2: %v", err)
	}
	if err := st.UpsertAlertSuppression(ctx, "backup_error:pve:"+itoa(server1), time.Now().Add(time.Hour), "test"); err != nil {
		t.Fatalf("upsert suppression: %v", err)
	}

	if err := st.HardDeleteServerDataForAPIKey(ctx, key1.ID, "duplicate-host"); err != nil {
		t.Fatalf("hard delete: %v", err)
	}

	assertCount(t, st, `SELECT COUNT(*) FROM pve_servers WHERE id = ?`, server1, 0)
	assertCount(t, st, `SELECT COUNT(*) FROM pve_alert_config WHERE server_id = ?`, server1, 0)
	assertCount(t, st, `SELECT COUNT(*) FROM alert_suppressions WHERE alert_id LIKE ?`, "%:pve:"+itoa(server1)+"%", 0)

	assertCount(t, st, `SELECT COUNT(*) FROM pve_servers WHERE id = ?`, server2, 1)
	assertCount(t, st, `SELECT COUNT(*) FROM pve_alert_config WHERE server_id = ?`, server2, 1)
}

func TestHardDeleteServerDataForAPIKey_DeletesLegacyServerWithSameHostname(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)

	key, err := st.CreateAPIKey(ctx, "nicolas", "duplicate-host", "")
	if err != nil {
		t.Fatalf("create key: %v", err)
	}
	boundServer, err := st.UpsertPVEServerForAPIKey(ctx, key.ID, "duplicate-host", "10.0.0.1", "", "0.0.91", "machine-1")
	if err != nil {
		t.Fatalf("upsert bound server: %v", err)
	}
	res, err := st.db.ExecContext(ctx, `INSERT INTO pve_servers (name, ip, client_version, machine_id) VALUES (?, ?, ?, ?)`, "duplicate-host", "10.0.0.2", "0.0.70", "machine-old")
	if err != nil {
		t.Fatalf("insert legacy server: %v", err)
	}
	legacyServer, _ := res.LastInsertId()
	if err := st.UpsertPVEAlertConfig(ctx, domain.PVEAlertConfig{ServerID: boundServer, BackupErr: intPtr(1)}); err != nil {
		t.Fatalf("upsert bound alert config: %v", err)
	}
	if err := st.UpsertPVEAlertConfig(ctx, domain.PVEAlertConfig{ServerID: legacyServer, BackupErr: intPtr(1)}); err != nil {
		t.Fatalf("upsert legacy alert config: %v", err)
	}

	if err := st.HardDeleteServerDataForAPIKey(ctx, key.ID, "duplicate-host"); err != nil {
		t.Fatalf("hard delete: %v", err)
	}

	assertCount(t, st, `SELECT COUNT(*) FROM pve_servers WHERE id = ?`, boundServer, 0)
	assertCount(t, st, `SELECT COUNT(*) FROM pve_servers WHERE id = ?`, legacyServer, 0)
	assertCount(t, st, `SELECT COUNT(*) FROM pve_alert_config WHERE server_id = ?`, legacyServer, 0)
}

func assertCount(t *testing.T, st *Store, query string, arg any, want int) {
	t.Helper()
	var got int
	if err := st.db.QueryRow(query, arg).Scan(&got); err != nil {
		t.Fatalf("count query %q: %v", query, err)
	}
	if got != want {
		t.Fatalf("count query %q got %d, want %d", query, got, want)
	}
}

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}
