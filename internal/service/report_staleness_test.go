package service

import (
	"context"
	"testing"
	"time"

	"probakgo/internal/domain"
)

// saturday is 2026-05-02, a known Saturday in UTC.
var saturday = time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC)

func newSvcAt(t *testing.T, now time.Time) (*ReportService, func(domain.CreateVMBackupConfigRequest) int64) {
	t.Helper()
	ctx := context.Background()
	_, st := openTestStore(t)
	svc := &ReportService{store: st, tz: time.UTC, now: func() time.Time { return now }}
	create := func(req domain.CreateVMBackupConfigRequest) int64 {
		id, err := st.CreateVMBackupConfig(ctx, "pve-01", req)
		if err != nil {
			t.Fatalf("create config: %v", err)
		}
		return id
	}
	return svc, create
}

func TestIsStaleForServer_NoConfig_FallsBackToIsStale(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	svc := NewReport(st, time.UTC)

	stale, _ := svc.IsStaleForServer(ctx, time.Now().Add(-25*time.Hour), "pve-noconfig")
	if !stale {
		t.Error("want stale=true: yesterday's report, no config")
	}
	stale2, _ := svc.IsStaleForServer(ctx, time.Now(), "pve-noconfig")
	if stale2 {
		t.Error("want stale=false: today's report, no config")
	}
}

func TestIsStaleForServer_WeekendNoStale_FreshFridayReport(t *testing.T) {
	svc, create := newSvcAt(t, saturday) // "now" = Saturday 10:00
	create(domain.CreateVMBackupConfigRequest{
		VMID: "100", VMName: "vm",
		Monday: true, Tuesday: true, Wednesday: true, Thursday: true, Friday: true,
	})

	// Report arrived Friday 22:00. Saturday 09:00 has passed, so the Friday report is required.
	fridayEvening := time.Date(2026, 5, 1, 22, 0, 0, 0, time.UTC)
	stale, reason := svc.IsStaleForServer(context.Background(), fridayEvening, "pve-01")
	if stale {
		t.Errorf("want stale=false: Fri-only schedule, Fri report, checked on Sat; got reason=%q", reason)
	}
}

func TestIsStaleForServer_WeekendStale_NoFridayReport(t *testing.T) {
	svc, create := newSvcAt(t, saturday) // "now" = Saturday 10:00
	create(domain.CreateVMBackupConfigRequest{
		VMID: "100", VMName: "vm",
		Monday: true, Tuesday: true, Wednesday: true, Thursday: true, Friday: true,
	})

	// No report since Thursday - Friday backup was missed
	thursdayOld := time.Date(2026, 4, 30, 20, 0, 0, 0, time.UTC)
	stale, _ := svc.IsStaleForServer(context.Background(), thursdayOld, "pve-01")
	if !stale {
		t.Error("want stale=true: Fri-only schedule, last report is Thursday, checked on Sat")
	}
}

func TestIsStaleForServer_GracePeriod_EarlyMorning(t *testing.T) {
	// "now" = Saturday 02:00 - before the Friday backup window closes at 09:00.
	earlysat := time.Date(2026, 5, 2, 2, 0, 0, 0, time.UTC)
	svc, create := newSvcAt(t, earlysat)
	create(domain.CreateVMBackupConfigRequest{
		VMID: "100", VMName: "vm",
		Monday: true, Tuesday: true, Wednesday: true, Thursday: true, Friday: true,
	})

	// Friday 00:00 + 33h = Saturday 09:00; now=02:00 is still inside the window.
	// So Friday is not yet "completed" → look further back → Thursday.
	// Report is from Thursday 20:00. Fri not checked yet, so check Thu.
	// Thu 00:00 + 33h = Fri 09:00 < Sat 02:00 → completed.
	// reportedAt (Thu 20:00) >= Thu 00:00 → not stale.
	thursdayEvening := time.Date(2026, 4, 30, 20, 0, 0, 0, time.UTC)
	stale, reason := svc.IsStaleForServer(context.Background(), thursdayEvening, "pve-01")
	if stale {
		t.Errorf("want stale=false: before 09:00 cutoff for Fri, Thu report covers Thu; got reason=%q", reason)
	}
}

func TestIsStaleForServer_StaleAtNineWhenPreviousNightReportMissing(t *testing.T) {
	nineSat := time.Date(2026, 5, 2, 9, 0, 0, 0, time.UTC)
	svc, create := newSvcAt(t, nineSat)
	create(domain.CreateVMBackupConfigRequest{
		VMID: "100", VMName: "vm",
		Monday: true, Tuesday: true, Wednesday: true, Thursday: true, Friday: true,
	})

	thursdayEvening := time.Date(2026, 4, 30, 20, 0, 0, 0, time.UTC)
	stale, _ := svc.IsStaleForServer(context.Background(), thursdayEvening, "pve-01")
	if !stale {
		t.Error("want stale=true: at 09:00 the previous night's Friday report is required")
	}
}

func TestIsStaleForServer_UsesServerExpectedFinishTime(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	tenSat := time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC)
	svc := &ReportService{store: st, tz: time.UTC, now: func() time.Time { return tenSat }}

	serverID, err := st.UpsertPVEServer(ctx, "pve-01", "10.0.0.1", "", "1.0", "")
	if err != nil {
		t.Fatalf("upsert server: %v", err)
	}
	finish := "11:00"
	if err := st.UpsertPVEAlertConfig(ctx, domain.PVEAlertConfig{
		ServerID:           serverID,
		ExpectedFinishTime: &finish,
	}); err != nil {
		t.Fatalf("upsert alert config: %v", err)
	}
	if _, err := st.CreateVMBackupConfig(ctx, "pve-01", domain.CreateVMBackupConfigRequest{
		VMID: "100", VMName: "vm", Friday: true,
	}); err != nil {
		t.Fatalf("create config: %v", err)
	}

	thursdayEvening := time.Date(2026, 4, 30, 20, 0, 0, 0, time.UTC)
	stale, reason := svc.IsStaleForServer(ctx, thursdayEvening, "pve-01")
	if stale {
		t.Errorf("want stale=false before custom 11:00 cutoff; got reason=%q", reason)
	}
}

func TestIsStaleForServer_StaleAfterServerExpectedFinishTime(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	elevenSat := time.Date(2026, 5, 2, 11, 0, 0, 0, time.UTC)
	svc := &ReportService{store: st, tz: time.UTC, now: func() time.Time { return elevenSat }}

	serverID, err := st.UpsertPVEServer(ctx, "pve-01", "10.0.0.1", "", "1.0", "")
	if err != nil {
		t.Fatalf("upsert server: %v", err)
	}
	finish := "11:00"
	if err := st.UpsertPVEAlertConfig(ctx, domain.PVEAlertConfig{
		ServerID:           serverID,
		ExpectedFinishTime: &finish,
	}); err != nil {
		t.Fatalf("upsert alert config: %v", err)
	}
	if _, err := st.CreateVMBackupConfig(ctx, "pve-01", domain.CreateVMBackupConfigRequest{
		VMID: "100", VMName: "vm", Friday: true,
	}); err != nil {
		t.Fatalf("create config: %v", err)
	}

	thursdayEvening := time.Date(2026, 4, 30, 20, 0, 0, 0, time.UTC)
	stale, _ := svc.IsStaleForServer(ctx, thursdayEvening, "pve-01")
	if !stale {
		t.Error("want stale=true at custom 11:00 cutoff")
	}
}

func TestIsStaleForServer_AllConfigsExcluded_FallsBack(t *testing.T) {
	ctx := context.Background()
	_, st := openTestStore(t)
	svc := NewReport(st, time.UTC)

	if _, err := st.CreateVMBackupConfig(ctx, "pve-ex", domain.CreateVMBackupConfigRequest{
		VMID: "100", VMName: "vm", Monday: true, Friday: true,
	}); err != nil {
		t.Fatalf("create config: %v", err)
	}
	if err := st.ToggleVMExclude(ctx, "pve-ex", "100"); err != nil {
		t.Fatalf("toggle exclude: %v", err)
	}

	// All excluded → no expected days → falls back to IsStale (yesterday = stale)
	stale, _ := svc.IsStaleForServer(context.Background(), time.Now().Add(-25*time.Hour), "pve-ex")
	if !stale {
		t.Error("want stale=true: all configs excluded, yesterday's report")
	}
}
