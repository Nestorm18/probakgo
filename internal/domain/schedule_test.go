package domain

import (
	"testing"
	"time"
)

func TestVMScheduledForDay(t *testing.T) {
	cases := []struct {
		day   time.Weekday
		field string
		cfg   VMBackupConfig
		want  bool
	}{
		{time.Monday, "Monday", VMBackupConfig{Monday: true}, true},
		{time.Tuesday, "Tuesday", VMBackupConfig{Tuesday: true}, true},
		{time.Wednesday, "Wednesday", VMBackupConfig{Wednesday: true}, true},
		{time.Thursday, "Thursday", VMBackupConfig{Thursday: true}, true},
		{time.Friday, "Friday", VMBackupConfig{Friday: true}, true},
		{time.Saturday, "Saturday", VMBackupConfig{Saturday: true}, true},
		{time.Sunday, "Sunday", VMBackupConfig{Sunday: true}, true},
		{time.Monday, "Monday not set", VMBackupConfig{Tuesday: true}, false},
		{time.Saturday, "Saturday not set", VMBackupConfig{Monday: true, Friday: true}, false},
	}
	for _, tc := range cases {
		t.Run(tc.field, func(t *testing.T) {
			got := VMScheduledForDay(tc.cfg, tc.day)
			if got != tc.want {
				t.Errorf("VMScheduledForDay(%s): want %v, got %v", tc.day, tc.want, got)
			}
		})
	}
}

func TestHasActiveVMBackupConfigs(t *testing.T) {
	if HasActiveVMBackupConfigs(nil) {
		t.Fatal("nil configs should not be active")
	}
	if HasActiveVMBackupConfigs([]VMBackupConfig{{VMID: "100", Monday: true, IsExcluded: true}}) {
		t.Fatal("excluded config should not be active")
	}
	if HasActiveVMBackupConfigs([]VMBackupConfig{{VMID: "100"}}) {
		t.Fatal("config without scheduled days should not be active")
	}
	if !HasActiveVMBackupConfigs([]VMBackupConfig{{VMID: "100", Monday: true}}) {
		t.Fatal("non-excluded config with a scheduled day should be active")
	}
}
