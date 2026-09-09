package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joho/godotenv"
)

func TestBackupEnvPrecedenceAndSecrets(t *testing.T) {
	dir := t.TempDir()
	first, second := filepath.Join(dir, "cwd.env"), filepath.Join(dir, "exe.env")
	if err := os.WriteFile(first, []byte("LOCAL_ONLY=yes\nSHARED=first\nSESSION_KEY=old\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("SHARED=second\nEXE_ONLY=yes\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SESSION_KEY", "current-session")
	t.Setenv("DATA_ENCRYPTION_KEY", "current-data")
	t.Setenv("UNRELATED_PRIVATE_TOKEN", "do-not-export")
	data, err := backupEnvFrom([]string{first, second})
	if err != nil {
		t.Fatal(err)
	}
	got, err := godotenv.Unmarshal(string(data))
	if err != nil {
		t.Fatal(err)
	}
	if got["SHARED"] != "first" || got["LOCAL_ONLY"] != "yes" || got["EXE_ONLY"] != "yes" || got["SESSION_KEY"] != "current-session" || got["DATA_ENCRYPTION_KEY"] != "current-data" {
		t.Fatal("incorrect configuration precedence")
	}
	if _, exists := got["UNRELATED_PRIVATE_TOKEN"]; exists {
		t.Fatal("unrelated process secret exported")
	}
}

func TestBackupEnvRefusesMissingKeys(t *testing.T) {
	t.Setenv("SESSION_KEY", "")
	t.Setenv("DATA_ENCRYPTION_KEY", "")
	if _, err := backupEnvFrom([]string{filepath.Join(t.TempDir(), ".env")}); err == nil {
		t.Fatal("accepted backup without restoration keys")
	}
}
