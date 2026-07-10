package service

import (
	"context"
	"testing"
	"time"

	"probakgo/internal/domain"
)

func TestRunCleanup_Disabled(t *testing.T) {
	ctx := context.Background()
	db, st := openTestStore(t)

	if err := st.UpsertEmailConfig(ctx, domain.EmailConfig{
		RetentionEnabled: false,
		RetentionMonths:  3,
		SendTime:         "08:00",
	}); err != nil {
		t.Fatalf("UpsertEmailConfig: %v", err)
	}

	serverID, _ := st.UpsertPVEServer(ctx, "pve-node", "10.0.0.1", "", "1.0", "")
	oldID, _ := st.InsertPVEReport(ctx, serverID, nil)
	if _, err := db.Exec("UPDATE pve_reports SET reported_at = ? WHERE id = ?",
		time.Now().AddDate(0, -6, 0), oldID); err != nil {
		t.Fatalf("backdate report: %v", err)
	}

	runCleanup(ctx, st)

	rep, err := st.GetLatestPVEReport(ctx, serverID)
	if err != nil {
		t.Fatalf("report should not be deleted when retention disabled: %v", err)
	}
	if rep.ID != oldID {
		t.Errorf("want old report ID %d to survive, got %d", oldID, rep.ID)
	}
}

func TestRunCleanup_DeletesOld(t *testing.T) {
	ctx := context.Background()
	db, st := openTestStore(t)

	if err := st.UpsertEmailConfig(ctx, domain.EmailConfig{
		RetentionEnabled: true,
		RetentionMonths:  1,
		SendTime:         "08:00",
	}); err != nil {
		t.Fatalf("UpsertEmailConfig: %v", err)
	}

	twoMonthsAgo := time.Now().AddDate(0, -2, 0)

	// PVE: old report (to be deleted) + current report (to survive)
	pveID, _ := st.UpsertPVEServer(ctx, "pve-node", "10.0.0.1", "", "1.0", "")
	oldPVE, _ := st.InsertPVEReport(ctx, pveID, nil)
	db.Exec("UPDATE pve_reports SET reported_at = ? WHERE id = ?", twoMonthsAgo, oldPVE)
	_, _ = st.InsertPVEReport(ctx, pveID, nil)

	// PBS: old report (to be deleted) + current report (to survive)
	pbsID, _ := st.UpsertPBSServer(ctx, "pbs-node", "10.0.0.2", "", "1.0", "")
	oldPBS, _ := st.InsertPBSReport(ctx, pbsID)
	if err := st.InsertPBSTask(ctx, oldPBS, domain.PBSTaskPayload{TaskType: "sync", JobID: "old", Status: "OK"}); err != nil {
		t.Fatalf("insert old PBS task: %v", err)
	}
	db.Exec("UPDATE pbs_reports SET reported_at = ? WHERE id = ?", twoMonthsAgo, oldPBS)
	_, _ = st.InsertPBSReport(ctx, pbsID)

	runCleanup(ctx, st)

	var count int
	db.QueryRow("SELECT COUNT(*) FROM pve_reports WHERE id = ?", oldPVE).Scan(&count)
	if count != 0 {
		t.Error("old PVE report should be deleted")
	}
	db.QueryRow("SELECT COUNT(*) FROM pbs_reports WHERE id = ?", oldPBS).Scan(&count)
	if count != 0 {
		t.Error("old PBS report should be deleted")
	}
	db.QueryRow("SELECT COUNT(*) FROM pbs_maintenance_tasks WHERE report_id = ?", oldPBS).Scan(&count)
	if count != 0 {
		t.Error("old PBS maintenance tasks should be deleted")
	}

	if _, err := st.GetLatestPVEReport(ctx, pveID); err != nil {
		t.Fatalf("current PVE report should remain: %v", err)
	}
	if _, err := st.GetLatestPBSReport(ctx, pbsID); err != nil {
		t.Fatalf("current PBS report should remain: %v", err)
	}
}

func TestRunCleanup_CanceledContextDoesNotDelete(t *testing.T) {
	ctx := context.Background()
	db, st := openTestStore(t)

	if err := st.UpsertEmailConfig(ctx, domain.EmailConfig{
		RetentionEnabled: true,
		RetentionMonths:  1,
		SendTime:         "08:00",
	}); err != nil {
		t.Fatalf("UpsertEmailConfig: %v", err)
	}

	serverID, _ := st.UpsertPVEServer(ctx, "pve-node", "10.0.0.1", "", "1.0", "")
	oldID, _ := st.InsertPVEReport(ctx, serverID, nil)
	if _, err := db.Exec("UPDATE pve_reports SET reported_at = ? WHERE id = ?",
		time.Now().AddDate(0, -2, 0), oldID); err != nil {
		t.Fatalf("backdate report: %v", err)
	}

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	runCleanup(canceled, st)

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM pve_reports WHERE id = ?", oldID).Scan(&count); err != nil {
		t.Fatalf("count report: %v", err)
	}
	if count != 1 {
		t.Fatal("cleanup should not delete reports after context cancellation")
	}
}
