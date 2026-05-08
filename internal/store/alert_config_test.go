package store

import (
	"testing"

	"probakgo/internal/domain"
)

func intPtr(v int) *int { return &v }

func TestPVEAlertConfig_NotFound(t *testing.T) {
	st := openTestDB(t)
	cfg, err := st.GetPVEAlertConfig(999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DiskPct != nil || cfg.StaleHours != nil || cfg.BackupErr != nil {
		t.Error("expected all nil fields when no row exists")
	}
}

func TestPVEAlertConfig_RoundTrip(t *testing.T) {
	st := openTestDB(t)
	serverID, _ := st.UpsertPVEServer("pve1", "1.2.3.4", "", "1.0", "")

	want := domain.PVEAlertConfig{
		ServerID:   serverID,
		DiskPct:    intPtr(90),
		StaleHours: intPtr(0),
		BackupErr:  intPtr(1),
	}
	if err := st.UpsertPVEAlertConfig(want); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := st.GetPVEAlertConfig(serverID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.DiskPct == nil || *got.DiskPct != 90 {
		t.Errorf("DiskPct: got %v, want 90", got.DiskPct)
	}
	if got.StaleHours == nil || *got.StaleHours != 0 {
		t.Errorf("StaleHours: got %v, want 0", got.StaleHours)
	}
	if got.BackupErr == nil || *got.BackupErr != 1 {
		t.Errorf("BackupErr: got %v, want 1", got.BackupErr)
	}
}

func TestPVEAlertConfig_NullFields(t *testing.T) {
	st := openTestDB(t)
	serverID, _ := st.UpsertPVEServer("pve2", "1.2.3.4", "", "1.0", "")

	cfg := domain.PVEAlertConfig{ServerID: serverID} // all nil
	if err := st.UpsertPVEAlertConfig(cfg); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := st.GetPVEAlertConfig(serverID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.DiskPct != nil || got.StaleHours != nil || got.BackupErr != nil {
		t.Error("expected all nil after upsert with nil fields")
	}
}

func TestPVEVMAlertConfig_UpsertDeleteRoundTrip(t *testing.T) {
	st := openTestDB(t)
	serverID, _ := st.UpsertPVEServer("pve3", "1.2.3.4", "", "1.0", "")

	cfg := domain.PVEVMAlertConfig{
		ServerID:  serverID,
		VMID:      101,
		BackupErr: intPtr(1),
		MinSizeMB: intPtr(500),
	}
	if err := st.UpsertPVEVMAlertConfig(cfg); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := st.GetPVEVMAlertConfigs(serverID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 config, got %d", len(got))
	}
	if got[0].VMID != 101 || *got[0].BackupErr != 1 || *got[0].MinSizeMB != 500 {
		t.Errorf("unexpected values: %+v", got[0])
	}

	if err := st.DeletePVEVMAlertConfig(serverID, 101); err != nil {
		t.Fatalf("delete: %v", err)
	}
	got, _ = st.GetPVEVMAlertConfigs(serverID)
	if len(got) != 0 {
		t.Errorf("expected 0 configs after delete, got %d", len(got))
	}
}

func TestPBSAlertConfig_NotFound(t *testing.T) {
	st := openTestDB(t)
	cfg, err := st.GetPBSAlertConfig(999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DiskPct != nil || cfg.DaysUntilFull != nil || cfg.StaleHours != nil {
		t.Error("expected nil fields when no row exists")
	}
	if !cfg.VerifyAlert {
		t.Error("expected VerifyAlert=true as default")
	}
}

func TestPBSAlertConfig_RoundTrip(t *testing.T) {
	st := openTestDB(t)
	serverID, _ := st.UpsertPBSServer("pbs1", "1.2.3.4", "", "1.0", "")

	want := domain.PBSAlertConfig{
		ServerID:      serverID,
		DiskPct:       intPtr(85),
		DaysUntilFull: intPtr(14),
		StaleHours:    intPtr(48),
		VerifyAlert:   false,
	}
	if err := st.UpsertPBSAlertConfig(want); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := st.GetPBSAlertConfig(serverID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.DiskPct == nil || *got.DiskPct != 85 {
		t.Errorf("DiskPct: got %v, want 85", got.DiskPct)
	}
	if got.DaysUntilFull == nil || *got.DaysUntilFull != 14 {
		t.Errorf("DaysUntilFull: got %v, want 14", got.DaysUntilFull)
	}
	if got.StaleHours == nil || *got.StaleHours != 48 {
		t.Errorf("StaleHours: got %v, want 48", got.StaleHours)
	}
	if got.VerifyAlert {
		t.Error("VerifyAlert: got true, want false")
	}
}
