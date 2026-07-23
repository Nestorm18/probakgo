package domain

import "testing"

func TestPVEBackupStatusSummary(t *testing.T) {
	tests := []struct {
		name     string
		tasks    []PVEBackupTask
		fallback string
		want     string
	}{
		{name: "fallback", fallback: "WARNINGS: 1", want: "WARNINGS: 1"},
		{name: "all ok", tasks: []PVEBackupTask{{Status: "OK"}, {Status: "ok"}}, want: "OK"},
		{name: "warning", tasks: []PVEBackupTask{{Status: "OK"}, {Status: "WARNINGS: 1"}}, want: "WARNINGS: 1"},
		{name: "failure wins", tasks: []PVEBackupTask{{Status: "WARNINGS: 1"}, {Status: "ERROR: disk full"}}, want: "ERROR: disk full"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PVEBackupStatusSummary(tt.tasks, tt.fallback); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPVEBackupStatusWarning(t *testing.T) {
	for _, status := range []string{"WARNING", "WARNINGS: 1", " warnings: 2 "} {
		if !PVEBackupStatusWarning(status) {
			t.Errorf("expected %q to be a warning", status)
		}
	}
	for _, status := range []string{"OK", "ERROR", "PARTIAL", ""} {
		if PVEBackupStatusWarning(status) {
			t.Errorf("did not expect %q to be a warning", status)
		}
	}
}
