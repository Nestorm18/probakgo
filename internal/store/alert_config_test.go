package store

import (
	"context"
	"testing"

	"probakgo/internal/domain"
)

func intPtr(v int) *int { return &v }

func stringPtr(v string) *string { return &v }

func TestPVEAlertConfig_NotFound(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	cfg, err := st.GetPVEAlertConfig(ctx, 999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DiskPct != nil || cfg.StaleHours != nil || cfg.BackupErr != nil || cfg.ExpectedFinishTime != nil {
		t.Error("expected all nil fields when no row exists")
	}
}

func TestPVEAlertConfig_RoundTrip(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	serverID, _ := st.UpsertPVEServer(ctx, "pve1", "1.2.3.4", "", "1.0", "")

	want := domain.PVEAlertConfig{
		ServerID:           serverID,
		DiskPct:            intPtr(90),
		StaleHours:         intPtr(0),
		BackupErr:          intPtr(1),
		ExpectedFinishTime: stringPtr("10:30"),
	}
	if err := st.UpsertPVEAlertConfig(ctx, want); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := st.GetPVEAlertConfig(ctx, serverID)
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
	if got.ExpectedFinishTime == nil || *got.ExpectedFinishTime != "10:30" {
		t.Errorf("ExpectedFinishTime: got %v, want 10:30", got.ExpectedFinishTime)
	}
}

func TestPVEAlertConfig_NullFields(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	serverID, _ := st.UpsertPVEServer(ctx, "pve2", "1.2.3.4", "", "1.0", "")

	cfg := domain.PVEAlertConfig{ServerID: serverID} // all nil
	if err := st.UpsertPVEAlertConfig(ctx, cfg); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := st.GetPVEAlertConfig(ctx, serverID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.DiskPct != nil || got.StaleHours != nil || got.BackupErr != nil || got.ExpectedFinishTime != nil {
		t.Error("expected all nil after upsert with nil fields")
	}
}

func TestPVEVMAlertConfig_UpsertDeleteRoundTrip(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	serverID, _ := st.UpsertPVEServer(ctx, "pve3", "1.2.3.4", "", "1.0", "")

	cfg := domain.PVEVMAlertConfig{
		ServerID:  serverID,
		VMID:      101,
		BackupErr: intPtr(1),
		MinSizeMB: intPtr(500),
	}
	if err := st.UpsertPVEVMAlertConfig(ctx, cfg); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := st.GetPVEVMAlertConfigs(ctx, serverID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 config, got %d", len(got))
	}
	if got[0].VMID != 101 || *got[0].BackupErr != 1 || *got[0].MinSizeMB != 500 {
		t.Errorf("unexpected values: %+v", got[0])
	}

	if err := st.DeletePVEVMAlertConfig(ctx, serverID, 101); err != nil {
		t.Fatalf("delete: %v", err)
	}
	got, _ = st.GetPVEVMAlertConfigs(ctx, serverID)
	if len(got) != 0 {
		t.Errorf("expected 0 configs after delete, got %d", len(got))
	}
}

func TestPBSAlertConfig_NotFound(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	cfg, err := st.GetPBSAlertConfig(ctx, 999)
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
	ctx := context.Background()
	st := openTestDB(t)
	serverID, _ := st.UpsertPBSServer(ctx, "pbs1", "1.2.3.4", "", "1.0", "")

	want := domain.PBSAlertConfig{
		ServerID:      serverID,
		DiskPct:       intPtr(85),
		DaysUntilFull: intPtr(14),
		StaleHours:    intPtr(48),
		VerifyAlert:   false,
	}
	if err := st.UpsertPBSAlertConfig(ctx, want); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := st.GetPBSAlertConfig(ctx, serverID)
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
