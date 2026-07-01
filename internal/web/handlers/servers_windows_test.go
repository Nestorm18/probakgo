package webhandlers

import (
	"context"
	"database/sql"
	"testing"
	"time"

	dbpkg "probakgo/internal/db"
	"probakgo/internal/domain"
	"probakgo/internal/store"
)

func TestWindowsAlertControls(t *testing.T) {
	disks := []windowsDiskDisplay{{
		WindowsDisk: domain.WindowsDisk{Name: "C:"},
	}}
	until := time.Now().Add(time.Hour)
	controls := windowsAlertControls(10, disks, map[string]time.Time{
		"disk:windows:10:C:": until,
	})
	if len(controls) != 3 {
		t.Fatalf("got %d controls, want 3", len(controls))
	}
	if controls[0].ID != "windows_heartbeat:windows:10" {
		t.Fatalf("heartbeat id = %q", controls[0].ID)
	}
	if controls[1].ID != "disk:windows:10:C:" || !controls[1].Suppressed {
		t.Fatalf("disk control = %+v", controls[1])
	}
	if controls[2].ID != "windows_disk_health:windows:10:C:" {
		t.Fatalf("health id = %q", controls[2].ID)
	}
}

func TestWindowsDiskChartDataUsesTemplateTimezone(t *testing.T) {
	ctx := context.Background()
	db, st := openWindowsHandlerDB(t)
	serverID, _ := st.UpsertWindowsServer(ctx, "win-chart", "1.1.1.1", "", "1.0", "machine-win")
	reportID, _ := st.InsertWindowsReport(ctx, serverID)
	_ = st.InsertWindowsDisk(ctx, reportID, domain.WindowsDiskPayload{Name: "C:", DriveType: "Fixed", Total: 1000, Used: 500, Free: 500})
	reportedAt := time.Date(2026, 7, 1, 10, 30, 0, 0, time.UTC)
	_, _ = db.Exec(`UPDATE windows_reports SET reported_at = ? WHERE id = ?`, reportedAt, reportID)
	reports, _ := st.ListWindowsReports(ctx, serverID, 1)
	loc := time.FixedZone("CEST", 2*60*60)
	h := &WebH{store: st, tmpl: &Templates{loc: loc}}

	points := h.windowsDiskChartData(ctx, reports)
	if len(points) != 1 {
		t.Fatalf("points: got %d, want 1", len(points))
	}
	if points[0].Label != "01/07 12:30" {
		t.Fatalf("label = %q, want local timezone label", points[0].Label)
	}
}

func openWindowsHandlerDB(t *testing.T) (*sql.DB, *store.Store) {
	t.Helper()
	db, err := dbpkg.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db, store.New(db)
}
