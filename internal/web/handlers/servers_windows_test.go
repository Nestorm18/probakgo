package webhandlers

import (
	"testing"
	"time"

	"probakgo/internal/domain"
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
