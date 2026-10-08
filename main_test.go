package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	dbpkg "probakgo/internal/db"
	"probakgo/internal/store"
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

func TestUnbanIPRejectsInvalidAddress(t *testing.T) {
	for _, raw := range []string{"", "not-an-ip", "10.0.0.256", "2001:db8::/129"} {
		if _, err := unbanIP(raw); err == nil {
			t.Errorf("unbanIP(%q) accepted an invalid address", raw)
		}
	}
}

func TestRequireExistingDatabaseDoesNotCreateIt(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "probakgo_data.db")
	if err := requireExistingDatabase(missing); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing database: got %v", err)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("database file was created: %v", err)
	}
	if err := os.WriteFile(missing, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := requireExistingDatabase(missing); err != nil {
		t.Fatalf("existing database rejected: %v", err)
	}
	if err := requireExistingDatabase(":memory:"); err != nil {
		t.Fatalf("in-memory database rejected: %v", err)
	}
}

func TestEnsureDefaultsReplacesStaleInitialPasswordFile(t *testing.T) {
	database, err := dbpkg.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	st := store.New(database)
	path := filepath.Join(t.TempDir(), ".initial-admin-password")
	if err := os.WriteFile(path, []byte("stale\n"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := ensureDefaultsAt(st, path); err != nil {
		t.Fatalf("ensureDefaultsAt with a stale password file: %v", err)
	}
	password, err := consumeInitialPassword(path)
	if err != nil || password == "stale" || len(password) != 16 {
		t.Fatalf("initial password not replaced: %q %v", password, err)
	}
	if hasUsers, err := st.HasUsers(context.Background()); err != nil || !hasUsers {
		t.Fatalf("default admin not created: %v %v", hasUsers, err)
	}
}
