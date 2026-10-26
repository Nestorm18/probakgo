package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHeartbeatTimerUsesCalendarSchedule(t *testing.T) {
	if !strings.Contains(heartbeatTimerUnit, "OnCalendar=*:0/5") {
		t.Fatalf("heartbeat timer must use a calendar schedule:\n%s", heartbeatTimerUnit)
	}
	// Monotonic OnUnitActiveSec stops rescheduling when the service is killed mid-run.
	if strings.Contains(heartbeatTimerUnit, "OnUnitActiveSec") || strings.Contains(heartbeatTimerUnit, "OnBootSec") {
		t.Fatalf("heartbeat timer must not depend on monotonic triggers:\n%s", heartbeatTimerUnit)
	}
}

func TestHeartbeatUnitsCurrent(t *testing.T) {
	dir := t.TempDir()
	servicePath := filepath.Join(dir, "heartbeat.service")
	timerPath := filepath.Join(dir, "heartbeat.timer")

	if heartbeatUnitsCurrent(servicePath, timerPath) {
		t.Fatal("missing units must not be current")
	}
	if err := os.WriteFile(servicePath, []byte(heartbeatServiceUnit), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(timerPath, []byte("[Timer]\nOnUnitActiveSec=5min\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if heartbeatUnitsCurrent(servicePath, timerPath) {
		t.Fatal("outdated timer must not be current")
	}
	if err := os.WriteFile(timerPath, []byte(heartbeatTimerUnit), 0644); err != nil {
		t.Fatal(err)
	}
	if !heartbeatUnitsCurrent(servicePath, timerPath) {
		t.Fatal("matching units must be current")
	}
}
