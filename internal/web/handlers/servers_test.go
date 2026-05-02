package webhandlers

import (
	"testing"
	"time"

	"probakgo/internal/domain"
)

func TestVmScheduledForDay(t *testing.T) {
	cases := []struct {
		day   time.Weekday
		field string
		cfg   domain.VMBackupConfig
		want  bool
	}{
		{time.Monday, "Monday", domain.VMBackupConfig{Monday: true}, true},
		{time.Tuesday, "Tuesday", domain.VMBackupConfig{Tuesday: true}, true},
		{time.Wednesday, "Wednesday", domain.VMBackupConfig{Wednesday: true}, true},
		{time.Thursday, "Thursday", domain.VMBackupConfig{Thursday: true}, true},
		{time.Friday, "Friday", domain.VMBackupConfig{Friday: true}, true},
		{time.Saturday, "Saturday", domain.VMBackupConfig{Saturday: true}, true},
		{time.Sunday, "Sunday", domain.VMBackupConfig{Sunday: true}, true},
		{time.Monday, "Monday not set", domain.VMBackupConfig{Tuesday: true}, false},
		{time.Saturday, "Saturday not set", domain.VMBackupConfig{Monday: true, Friday: true}, false},
	}
	for _, tc := range cases {
		t.Run(tc.field, func(t *testing.T) {
			got := vmScheduledForDay(tc.cfg, tc.day)
			if got != tc.want {
				t.Errorf("vmScheduledForDay(%s): want %v, got %v", tc.day, tc.want, got)
			}
		})
	}
}
