package webhandlers

import (
	"net/http"
	"net/http/httptest"
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
