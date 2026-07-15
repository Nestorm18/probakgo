package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReplaceInstalledBinary(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "installer.exe")
	dst := filepath.Join(dir, "installed.exe")
	if err := os.WriteFile(src, []byte("new-version"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("old-version"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := replaceInstalledBinary(src, dst); err != nil {
		t.Fatalf("replaceInstalledBinary: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new-version" {
		t.Fatalf("installed content = %q, want new-version", got)
	}
}

func TestReplaceInstalledBinaryDoesNotTruncateItself(t *testing.T) {
	path := filepath.Join(t.TempDir(), "installed.exe")
	if err := os.WriteFile(path, []byte("same-version"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := replaceInstalledBinary(path, path); err != nil {
		t.Fatalf("replaceInstalledBinary same path: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "same-version" {
		t.Fatalf("installed content = %q, want same-version", got)
	}
}
