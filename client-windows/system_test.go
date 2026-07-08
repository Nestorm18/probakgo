package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseDisksJSON_Array(t *testing.T) {
	got, err := parseDisksJSON([]byte(`[{"Name":"C:","Label":"System","FileSystem":"NTFS","DriveType":"Fixed","Total":1000,"Used":750,"Free":250,"Health":""},{"Name":"Physical 0","Label":"SSD","FileSystem":"","DriveType":"Physical","Total":1000,"Used":0,"Free":0,"Health":"OK"}]`))
	if err != nil {
		t.Fatalf("parseDisksJSON: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 logical disk, got %d", len(got))
	}
	if got[0].Name != "C:" || got[0].Used != 750 {
		t.Fatalf("unexpected disks: %#v", got)
	}
}

func TestParseDisksJSON_SingleObject(t *testing.T) {
	got, err := parseDisksJSON([]byte(`{"Name":"C:","Label":"","FileSystem":"NTFS","DriveType":"Fixed","Total":1000,"Used":100,"Free":900,"Health":""}`))
	if err != nil {
		t.Fatalf("parseDisksJSON: %v", err)
	}
	if len(got) != 1 || got[0].Name != "C:" {
		t.Fatalf("unexpected disks: %#v", got)
	}
}

func TestAPIURL(t *testing.T) {
	cases := []struct {
		base string
		path string
		want string
	}{
		{"https://probakgo.example", "/api/report/windows", "https://probakgo.example/api/report/windows"},
		{"https://probakgo.example/api", "/api/report/windows", "https://probakgo.example/api/report/windows"},
		{"192.168.1.10:36748", "/api/report/windows", "http://192.168.1.10:36748/api/report/windows"},
	}
	for _, tc := range cases {
		if got := apiURL(normalizeAPIURL(tc.base), tc.path); got != tc.want {
			t.Fatalf("apiURL(%q, %q) = %q, want %q", tc.base, tc.path, got, tc.want)
		}
	}
}

func TestNormalizeAPIURL(t *testing.T) {
	cases := map[string]string{
		"192.168.1.10:36748":        "http://192.168.1.10:36748",
		"http://192.168.1.10:36748": "http://192.168.1.10:36748",
		"https://probakgo.example/": "https://probakgo.example",
	}
	for in, want := range cases {
		if got := normalizeAPIURL(in); got != want {
			t.Fatalf("normalizeAPIURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLoadEnvIntoProcessLoadsGitHubToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("API_URL=http://probakgo.example\nGITHUB_TOKEN=\"expired-token\"\n"), 0600); err != nil {
		t.Fatalf("write env: %v", err)
	}
	if err := loadEnvIntoProcess(path); err != nil {
		t.Fatalf("loadEnvIntoProcess: %v", err)
	}
	if got := os.Getenv("GITHUB_TOKEN"); got != "expired-token" {
		t.Fatalf("GITHUB_TOKEN: got %q, want expired-token", got)
	}
}
