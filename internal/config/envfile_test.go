package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/joho/godotenv"
)

func TestSetDotEnvValue(t *testing.T) {
	got := SetDotEnvValue("SESSION_KEY=abc\nSESSION_SECURE=false\nAPI_PORT=36748\n", "SESSION_SECURE", "true")
	want := "SESSION_KEY=abc\nSESSION_SECURE=true\nAPI_PORT=36748\n"
	if got != want {
		t.Fatalf("update existing:\nwant %q\ngot  %q", want, got)
	}

	got = SetDotEnvValue("SESSION_KEY=abc\n", "SESSION_SECURE", "true")
	want = "SESSION_KEY=abc\nSESSION_SECURE=true\n"
	if got != want {
		t.Fatalf("append missing:\nwant %q\ngot  %q", want, got)
	}

	got = SetDotEnvValue("# SESSION_SECURE=false\n", "SESSION_SECURE", "true")
	want = "# SESSION_SECURE=false\nSESSION_SECURE=true\n"
	if got != want {
		t.Fatalf("ignore comments:\nwant %q\ngot  %q", want, got)
	}
}

func TestSetEnvFileValueKeepsKeysWithoutTrailingNewline(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("API_PORT=36748"), 0600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	if err := SetEnvFileValue(path, "DATA_ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef"); err != nil {
		t.Fatalf("SetEnvFileValue: %v", err)
	}
	values, err := godotenv.Read(path)
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}
	if values["API_PORT"] != "36748" || values["DATA_ENCRYPTION_KEY"] != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("values: got %+v", values)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat .env: %v", err)
	}
	if got := info.Mode().Perm(); runtime.GOOS != "windows" && got != 0600 {
		t.Fatalf("mode: got %o, want 600", got)
	}
	leftovers, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".env-*.tmp"))
	if err != nil || len(leftovers) != 0 {
		t.Fatalf("temporary files left behind: %v %v", leftovers, err)
	}
}

func TestSetEnvFileValueCreatesMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := SetEnvFileValue(path, "SESSION_KEY", "value"); err != nil {
		t.Fatalf("SetEnvFileValue: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}
	if string(data) != "SESSION_KEY=value\n" {
		t.Fatalf("content: got %q", data)
	}
}
