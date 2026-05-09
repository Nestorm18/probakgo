package store

import (
	"context"
	"testing"

	"probakgo/internal/domain"
)

func TestCreateAndListVMBackupConfig(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)

	req := domain.CreateVMBackupConfigRequest{
		VMID:      "100",
		VMName:    "web-server",
		Monday:    true,
		Wednesday: true,
		Friday:    true,
	}
	id, err := st.CreateVMBackupConfig(ctx, "pve-01", req)
	if err != nil {
		t.Fatalf("CreateVMBackupConfig: %v", err)
	}
	if id == 0 {
		t.Error("want non-zero ID")
	}

	configs, err := st.ListVMBackupConfigs(ctx, "pve-01")
	if err != nil {
		t.Fatalf("ListVMBackupConfigs: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("want 1 config, got %d", len(configs))
	}
	c := configs[0]
	if c.VMID != "100" {
		t.Errorf("VMID: want 100, got %q", c.VMID)
	}
	if c.VMName != "web-server" {
		t.Errorf("VMName: want web-server, got %q", c.VMName)
	}
	if !c.Monday || !c.Wednesday || !c.Friday {
		t.Errorf("days: want Mon/Wed/Fri true, got Mon=%v Wed=%v Fri=%v", c.Monday, c.Wednesday, c.Friday)
	}
	if c.Tuesday || c.Thursday || c.Saturday || c.Sunday {
		t.Error("unexpected days set to true")
	}

	// Different server → empty list
	others, _ := st.ListVMBackupConfigs(ctx, "pve-99")
	if len(others) != 0 {
		t.Errorf("want 0 configs for other server, got %d", len(others))
	}
}

func TestUpdateVMBackupConfig(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)

	_, _ = st.CreateVMBackupConfig(ctx, "pve-01", domain.CreateVMBackupConfigRequest{
		VMID: "101", VMName: "old-name", Monday: true,
	})

	if err := st.UpdateVMBackupConfig(ctx, "pve-01", "101", domain.CreateVMBackupConfigRequest{
		VMID: "101", VMName: "new-name", Tuesday: true, Thursday: true,
	}); err != nil {
		t.Fatalf("UpdateVMBackupConfig: %v", err)
	}

	configs, _ := st.ListVMBackupConfigs(ctx, "pve-01")
	if len(configs) != 1 {
		t.Fatalf("want 1 config, got %d", len(configs))
	}
	c := configs[0]
	if c.VMName != "new-name" {
		t.Errorf("VMName: want new-name, got %q", c.VMName)
	}
	if c.Monday {
		t.Error("Monday should be cleared after update")
	}
	if !c.Tuesday || !c.Thursday {
		t.Error("want Tuesday and Thursday true after update")
	}
}

func TestDeleteVMBackupConfig_SoftDelete(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)

	_, _ = st.CreateVMBackupConfig(ctx, "pve-01", domain.CreateVMBackupConfigRequest{
		VMID: "200", VMName: "vm-to-delete",
	})

	if err := st.DeleteVMBackupConfig(ctx, "pve-01", "200"); err != nil {
		t.Fatalf("DeleteVMBackupConfig: %v", err)
	}

	configs, err := st.ListVMBackupConfigs(ctx, "pve-01")
	if err != nil {
		t.Fatalf("ListVMBackupConfigs after delete: %v", err)
	}
	if len(configs) != 0 {
		t.Errorf("want 0 configs after soft delete, got %d", len(configs))
	}

	// Row must still exist in DB (soft delete)
	var count int
	st.db.QueryRow("SELECT COUNT(*) FROM vm_backup_configs WHERE vm_id='200' AND is_deleted=1").Scan(&count)
	if count != 1 {
		t.Error("want soft-deleted row to remain in DB with is_deleted=1")
	}
}

func TestToggleVMExclude(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	_, _ = st.CreateVMBackupConfig(ctx, "pve-01", domain.CreateVMBackupConfigRequest{VMID: "300", VMName: "vm"})

	configs, _ := st.ListVMBackupConfigs(ctx, "pve-01")
	if configs[0].IsExcluded {
		t.Fatal("want IsExcluded=false initially")
	}

	if err := st.ToggleVMExclude(ctx, "pve-01", "300"); err != nil {
		t.Fatalf("first ToggleVMExclude: %v", err)
	}
	configs, _ = st.ListVMBackupConfigs(ctx, "pve-01")
	if !configs[0].IsExcluded {
		t.Error("want IsExcluded=true after first toggle")
	}

	if err := st.ToggleVMExclude(ctx, "pve-01", "300"); err != nil {
		t.Fatalf("second ToggleVMExclude: %v", err)
	}
	configs, _ = st.ListVMBackupConfigs(ctx, "pve-01")
	if configs[0].IsExcluded {
		t.Error("want IsExcluded=false after second toggle")
	}
}
