package service

import (
	"testing"
	"time"

	"probakgo/internal/domain"
)

func TestRunCleanup_Disabled(t *testing.T) {
	db, st := openTestStore(t)

	if err := st.UpsertEmailConfig(domain.EmailConfig{
		RetentionEnabled: false,
		RetentionMonths:  3,
		SendTime:         "08:00",
	}); err != nil {
		t.Fatalf("UpsertEmailConfig: %v", err)
	}

	serverID, _ := st.UpsertPVEServer("pve-node", "10.0.0.1", "", "1.0", "")
	oldID, _ := st.InsertPVEReport(serverID, nil)
	if _, err := db.Exec("UPDATE pve_reports SET reported_at = ? WHERE id = ?",
		time.Now().AddDate(0, -6, 0), oldID); err != nil {
		t.Fatalf("backdate report: %v", err)
	}

	runCleanup(st)

	rep, err := st.GetLatestPVEReport(serverID)
	if err != nil {
		t.Fatalf("report should not be deleted when retention disabled: %v", err)
	}
	if rep.ID != oldID {
		t.Errorf("want old report ID %d to survive, got %d", oldID, rep.ID)
	}
}

func TestRunCleanup_DeletesOld(t *testing.T) {
	db, st := openTestStore(t)

	if err := st.UpsertEmailConfig(domain.EmailConfig{
		RetentionEnabled: true,
		RetentionMonths:  1,
		SendTime:         "08:00",
	}); err != nil {
		t.Fatalf("UpsertEmailConfig: %v", err)
	}

	twoMonthsAgo := time.Now().AddDate(0, -2, 0)

	// PVE: old report (to be deleted) + current report (to survive)
	pveID, _ := st.UpsertPVEServer("pve-node", "10.0.0.1", "", "1.0", "")
	oldPVE, _ := st.InsertPVEReport(pveID, nil)
	db.Exec("UPDATE pve_reports SET reported_at = ? WHERE id = ?", twoMonthsAgo, oldPVE)
	_, _ = st.InsertPVEReport(pveID, nil)

	// PBS: old report (to be deleted) + current report (to survive)
	pbsID, _ := st.UpsertPBSServer("pbs-node", "10.0.0.2", "", "1.0", "")
	oldPBS, _ := st.InsertPBSReport(pbsID)
	db.Exec("UPDATE pbs_reports SET reported_at = ? WHERE id = ?", twoMonthsAgo, oldPBS)
	_, _ = st.InsertPBSReport(pbsID)

	runCleanup(st)

	var count int
	db.QueryRow("SELECT COUNT(*) FROM pve_reports WHERE id = ?", oldPVE).Scan(&count)
	if count != 0 {
		t.Error("old PVE report should be deleted")
	}
	db.QueryRow("SELECT COUNT(*) FROM pbs_reports WHERE id = ?", oldPBS).Scan(&count)
	if count != 0 {
		t.Error("old PBS report should be deleted")
	}

	if _, err := st.GetLatestPVEReport(pveID); err != nil {
		t.Fatalf("current PVE report should remain: %v", err)
	}
	if _, err := st.GetLatestPBSReport(pbsID); err != nil {
		t.Fatalf("current PBS report should remain: %v", err)
	}
}
