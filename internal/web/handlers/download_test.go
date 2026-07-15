package webhandlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGitHubDownloadToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/download/client/linux-amd64", nil)
	req.Header.Set("X-GitHub-Token", "ghp_header")
	if got := githubDownloadToken(req); got != "ghp_header" {
		t.Fatalf("header token: got %q", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/download/client/linux-amd64", nil)
	req.Header.Set("Authorization", "Bearer ghp_auth")
	if got := githubDownloadToken(req); got != "ghp_auth" {
		t.Fatalf("authorization token: got %q", got)
	}
}

func TestDownloadResponseLimits(t *testing.T) {
	var decoded map[string]string
	if err := decodeJSONWithLimit(strings.NewReader(`{"tag":"ok"}`), &decoded, 32); err != nil {
		t.Fatalf("decode within limit: %v", err)
	}
	if decoded["tag"] != "ok" {
		t.Fatalf("decoded tag = %q", decoded["tag"])
	}
	if err := decodeJSONWithLimit(strings.NewReader(`{"tag":"too-large"}`), &decoded, 8); err == nil {
		t.Fatal("expected oversized JSON to fail")
	}

	var dst bytes.Buffer
	if _, err := copyWithLimit(&dst, strings.NewReader("12345"), 5); err != nil {
		t.Fatalf("copy at limit: %v", err)
	}
	if _, err := copyWithLimit(&dst, strings.NewReader("123456"), 5); err == nil {
		t.Fatal("expected oversized copy to fail")
	}
}
