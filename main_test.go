package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRandomPasswordUsesExpectedAlphabetAndLength(t *testing.T) {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	for range 100 {
		password, err := randomPassword()
		if err != nil {
			t.Fatalf("randomPassword: %v", err)
		}
		if len(password) != 16 {
			t.Fatalf("length: got %d, want 16", len(password))
		}
		for _, char := range password {
			if !strings.ContainsRune(alphabet, char) {
				t.Fatalf("unexpected character %q in %q", char, password)
			}
		}
	}
}

func TestInitialPasswordFileIsSingleUse(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".initial-admin-password")

	if err := writeInitialPassword(path, "secret-value"); err != nil {
		t.Fatalf("writeInitialPassword: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat password file: %v", err)
	}
	if got := info.Mode().Perm(); runtime.GOOS != "windows" && got != 0600 {
		t.Fatalf("mode: got %o, want 600", got)
	}

	password, err := consumeInitialPassword(path)
	if err != nil {
		t.Fatalf("consumeInitialPassword: %v", err)
	}
	if password != "secret-value" {
		t.Fatalf("password: got %q", password)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("password file still exists: %v", err)
	}
	if _, err := consumeInitialPassword(path); !os.IsNotExist(err) {
		t.Fatalf("second consume error: got %v, want not-exist", err)
	}
}

func TestSystemdServiceContentIsHardened(t *testing.T) {
	content := systemdServiceContent("/opt/probakgo/probakgo", "/opt/probakgo", "probakgo")
	for _, expected := range []string{
		"User=probakgo",
		"Group=probakgo",
		"NoNewPrivileges=true",
		"PrivateTmp=true",
		"ProtectSystem=strict",
		"ReadWritePaths=/opt/probakgo",
		"Restart=always",
	} {
		if !strings.Contains(content, expected) {
			t.Errorf("service unit does not contain %q", expected)
		}
	}
}
