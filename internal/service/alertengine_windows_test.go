package service

import (
	"context"
	"testing"
	"time"

	"probakgo/internal/domain"
)

func TestEvalWindowsDisk_OverThreshold(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	serverID, _ := st.UpsertWindowsServer(ctx, "win1", "1.1.1.1", "", "1.0", "machine-win")
	reportID, _ := st.InsertWindowsReport(ctx, serverID)
	_ = st.InsertWindowsDisk(ctx, reportID, domain.WindowsDiskPayload{
		Name: "C:", Total: 1000, Used: 900, Free: 100,
	})

	alerts, err := evalWindowsDisk(st, defaultCfg())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasAlert(alerts, domain.AlertTypeDisk, "win1") {
		t.Error("expected windows disk alert for win1")
	}
}

func TestEvalWindowsDisk_IgnoresPhysicalDiskRows(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	serverID, _ := st.UpsertWindowsServer(ctx, "win1", "1.1.1.1", "", "1.0", "machine-win")
	reportID, _ := st.InsertWindowsReport(ctx, serverID)
	_ = st.InsertWindowsDisk(ctx, reportID, domain.WindowsDiskPayload{
		Name: "Physical 0", DriveType: "Physical", Total: 1000, Used: 990, Free: 10,
	})

	alerts, err := evalWindowsDisk(st, defaultCfg())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasAlert(alerts, domain.AlertTypeDisk, "win1") {
		t.Error("did not expect physical disk row to create windows disk alert")
	}
}

func TestEvalWindowsDisk_UsesServerOverride(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	serverID, _ := st.UpsertWindowsServer(ctx, "win-override", "1.1.1.1", "", "1.0", "machine-win")
	reportID, _ := st.InsertWindowsReport(ctx, serverID)
	_ = st.InsertWindowsDisk(ctx, reportID, domain.WindowsDiskPayload{
		Name: "C:", Total: 1000, Used: 900, Free: 100,
	})

	threshold := 95
	cfg := defaultCfg()
	cfg.WindowsConfigs[serverID] = domain.WindowsAlertConfig{ServerID: serverID, DiskPct: &threshold}
	alerts, err := evalWindowsDisk(st, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasAlert(alerts, domain.AlertTypeDisk, "win-override") {
		t.Error("did not expect windows disk alert below server override")
	}
}

func TestEvalWindowsDisk_ServerOverrideCanEnableWhenGlobalDisabled(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	serverID, _ := st.UpsertWindowsServer(ctx, "win-enabled", "1.1.1.1", "", "1.0", "machine-win")
	reportID, _ := st.InsertWindowsReport(ctx, serverID)
	_ = st.InsertWindowsDisk(ctx, reportID, domain.WindowsDiskPayload{
		Name: "C:", Total: 1000, Used: 900, Free: 100,
	})

	threshold := 80
	cfg := defaultCfg()
	cfg.GlobalWindowsDiskPct = 0
	cfg.WindowsConfigs[serverID] = domain.WindowsAlertConfig{ServerID: serverID, DiskPct: &threshold}
	alerts, err := evalWindowsDisk(st, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasAlert(alerts, domain.AlertTypeDisk, "win-enabled") {
		t.Error("expected windows disk alert from server override")
	}
}

func TestEvalWindowsHeartbeat_Offline(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	serverID, _ := st.UpsertWindowsServer(ctx, "win-offline", "1.1.1.1", "", "1.0", "machine-win")
	_ = st.UpsertServerHeartbeat(ctx, domain.ServerHeartbeat{
		ServerType: "windows",
		ServerID:   serverID,
		Hostname:   "win-offline",
		LastSeenAt: time.Now().Add(-30 * time.Minute),
	})

	alerts, err := evalWindowsHeartbeat(st, defaultCfg())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasAlert(alerts, domain.AlertTypeWindowsHeartbeat, "win-offline") {
		t.Error("expected windows heartbeat alert for win-offline")
	}
}

func TestEvalWindowsMissingVolume_WhenPreviousLogicalDiskDisappears(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	serverID, _ := st.UpsertWindowsServer(ctx, "win-missing-volume", "1.1.1.1", "", "1.0", "machine-win")
	previousID, _ := st.InsertWindowsReport(ctx, serverID)
	_ = st.InsertWindowsDisk(ctx, previousID, domain.WindowsDiskPayload{Name: "C:", DriveType: "Fixed", Total: 1000, Used: 500, Free: 500})
	_ = st.InsertWindowsDisk(ctx, previousID, domain.WindowsDiskPayload{Name: "D:", DriveType: "Fixed", Total: 1000, Used: 100, Free: 900})
	_ = st.InsertWindowsDisk(ctx, previousID, domain.WindowsDiskPayload{Name: "Physical 0", DriveType: "Physical", Total: 1000})

	latestID, _ := st.InsertWindowsReport(ctx, serverID)
	_ = st.InsertWindowsDisk(ctx, latestID, domain.WindowsDiskPayload{Name: "C:", DriveType: "Fixed", Total: 1000, Used: 600, Free: 400})

	alerts, err := evalWindowsMissingVolume(st, defaultCfg())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasAlert(alerts, domain.AlertTypeWindowsVolumeGone, "win-missing-volume") {
		t.Fatal("expected missing volume alert")
	}
	if len(alerts) != 1 || alerts[0].StoreName != "D:" {
		t.Fatalf("expected only D: missing, got %+v", alerts)
	}
}

func TestEvalWindowsMissingVolume_NoAlertWhenVolumeStillPresent(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	serverID, _ := st.UpsertWindowsServer(ctx, "win-volume-ok", "1.1.1.1", "", "1.0", "machine-win")
	previousID, _ := st.InsertWindowsReport(ctx, serverID)
	_ = st.InsertWindowsDisk(ctx, previousID, domain.WindowsDiskPayload{Name: "D:", DriveType: "Fixed", Total: 1000, Used: 100, Free: 900})

	latestID, _ := st.InsertWindowsReport(ctx, serverID)
	_ = st.InsertWindowsDisk(ctx, latestID, domain.WindowsDiskPayload{Name: "D:", DriveType: "Fixed", Total: 1000, Used: 200, Free: 800})

	alerts, err := evalWindowsMissingVolume(st, defaultCfg())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasAlert(alerts, domain.AlertTypeWindowsVolumeGone, "win-volume-ok") {
		t.Fatal("did not expect missing volume alert")
	}
}
