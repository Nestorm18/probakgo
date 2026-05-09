package service

import (
	"testing"
	"time"

	"probakgo/internal/domain"
)

// saturday is 2026-05-02, a known Saturday in UTC.
var saturday = time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC)

func newSvcAt(t *testing.T, now time.Time) (*ReportService, func(domain.CreateVMBackupConfigRequest) int64) {
	t.Helper()
	_, st := openTestStore(t)
	svc := &ReportService{store: st, tz: time.UTC, now: func() time.Time { return now }}
	create := func(req domain.CreateVMBackupConfigRequest) int64 {
		id, err := st.CreateVMBackupConfig("pve-01", req)
		if err != nil {
			t.Fatalf("create config: %v", err)
		}
		return id
	}
	return svc, create
}

func TestIsStaleForServer_NoConfig_FallsBackToIsStale(t *testing.T) {
	_, st := openTestStore(t)
	svc := NewReport(st, time.UTC)

	stale, _ := svc.IsStaleForServer(time.Now().Add(-25*time.Hour), "pve-noconfig")
	if !stale {
		t.Error("want stale=true: yesterday's report, no config")
	}
	stale2, _ := svc.IsStaleForServer(time.Now(), "pve-noconfig")
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

	// Report arrived Friday 22:00 - Saturday 04:00 (28h after Fri 00:00) has passed, report covers Friday
	fridayEvening := time.Date(2026, 5, 1, 22, 0, 0, 0, time.UTC)
	stale, reason := svc.IsStaleForServer(fridayEvening, "pve-01")
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
	stale, _ := svc.IsStaleForServer(thursdayOld, "pve-01")
	if !stale {
		t.Error("want stale=true: Fri-only schedule, last report is Thursday, checked on Sat")
	}
}

func TestIsStaleForServer_GracePeriod_EarlyMorning(t *testing.T) {
	// "now" = Saturday 02:00 - within the 28h grace window of Friday's backup
	earlysat := time.Date(2026, 5, 2, 2, 0, 0, 0, time.UTC)
	svc, create := newSvcAt(t, earlysat)
	create(domain.CreateVMBackupConfigRequest{
		VMID: "100", VMName: "vm",
		Monday: true, Tuesday: true, Wednesday: true, Thursday: true, Friday: true,
	})

	// Friday 00:00 + 28h = Saturday 04:00; now=02:00 is still inside the window.
	// So Friday is not yet "completed" → look further back → Thursday.
	// Report is from Thursday 20:00. Fri not checked yet, so check Thu.
	// Thu 00:00 + 28h = Fri 04:00 < Sat 02:00 → completed.
	// reportedAt (Thu 20:00) >= Thu 00:00 → not stale.
	thursdayEvening := time.Date(2026, 4, 30, 20, 0, 0, 0, time.UTC)
	stale, reason := svc.IsStaleForServer(thursdayEvening, "pve-01")
	if stale {
		t.Errorf("want stale=false: within 28h grace for Fri, Thu report covers Thu; got reason=%q", reason)
	}
}

func TestIsStaleForServer_AllConfigsExcluded_FallsBack(t *testing.T) {
	_, st := openTestStore(t)
	svc := NewReport(st, time.UTC)

	if _, err := st.CreateVMBackupConfig("pve-ex", domain.CreateVMBackupConfigRequest{
		VMID: "100", VMName: "vm", Monday: true, Friday: true,
	}); err != nil {
		t.Fatalf("create config: %v", err)
	}
	if err := st.ToggleVMExclude("pve-ex", "100"); err != nil {
		t.Fatalf("toggle exclude: %v", err)
	}

	// All excluded → no expected days → falls back to IsStale (yesterday = stale)
	stale, _ := svc.IsStaleForServer(time.Now().Add(-25*time.Hour), "pve-ex")
	if !stale {
		t.Error("want stale=true: all configs excluded, yesterday's report")
	}
}
