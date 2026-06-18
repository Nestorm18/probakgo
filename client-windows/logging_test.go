package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPrepareLogFileRotatesPreviousDay(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ProgramData", root)
	if err := os.MkdirAll(installDir(), 0755); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 6, 18, 10, 0, 0, 0, time.Local)
	prev := now.AddDate(0, 0, -1)
	if err := os.WriteFile(logPath(), []byte("old log\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(logPath(), prev, prev); err != nil {
		t.Fatal(err)
	}
	if err := prepareLogFile(now); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(logPath()); !os.IsNotExist(err) {
		t.Fatalf("current log should be rotated, stat err=%v", err)
	}
	archive := filepath.Join(installDir(), "probakgo-windows-client-2026-06-17.log")
	data, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "old log") {
		t.Fatalf("archive does not contain previous log: %q", string(data))
	}
}

func TestPrepareLogFileKeepsOnlySevenDays(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ProgramData", root)
	if err := os.MkdirAll(installDir(), 0755); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 6, 18, 10, 0, 0, 0, time.Local)
	for daysAgo := 1; daysAgo <= 8; daysAgo++ {
		day := now.AddDate(0, 0, -daysAgo)
		name := filepath.Join(installDir(), "probakgo-windows-client-"+day.Format("2006-01-02")+".log")
		if err := os.WriteFile(name, []byte(day.Format("2006-01-02")), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := prepareLogFile(now); err != nil {
		t.Fatal(err)
	}
	for daysAgo := 1; daysAgo <= 6; daysAgo++ {
		day := now.AddDate(0, 0, -daysAgo)
		name := filepath.Join(installDir(), "probakgo-windows-client-"+day.Format("2006-01-02")+".log")
		if _, err := os.Stat(name); err != nil {
			t.Fatalf("expected %s to be kept: %v", name, err)
		}
	}
	for daysAgo := 7; daysAgo <= 8; daysAgo++ {
		day := now.AddDate(0, 0, -daysAgo)
		name := filepath.Join(installDir(), "probakgo-windows-client-"+day.Format("2006-01-02")+".log")
		if _, err := os.Stat(name); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be removed, stat err=%v", name, err)
		}
	}
}
