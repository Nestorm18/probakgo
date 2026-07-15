package selfupdate

import (
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name    string
		remote  string
		current string
		want    int
		wantOK  bool
	}{
		{"newer patch", "0.0.51", "0.0.50", 1, true},
		{"remote with v prefix", "v0.0.51", "0.0.50", 1, true},
		{"same with prefix", "v0.0.50", "0.0.50", 0, true},
		{"older remote", "0.0.49", "0.0.50", -1, true},
		{"local has extra zero", "0.0.50", "0.0.50.0", 0, true},
		{"pre release suffix ignored", "0.0.51-beta.1", "0.0.50", 1, true},
		{"invalid remote", "latest", "0.0.50", 0, false},
		{"invalid current", "0.0.51", "local", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := compareVersions(tt.remote, tt.current)
			if ok != tt.wantOK || got != tt.want {
				t.Fatalf("compareVersions(%q, %q) = %d, %v; want %d, %v", tt.remote, tt.current, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestIsNewer(t *testing.T) {
	newer, ok := IsNewer("v0.0.59", "0.0.58")
	if !ok || !newer {
		t.Fatalf("IsNewer newer = %v, %v; want true, true", newer, ok)
	}

	newer, ok = IsNewer("v0.0.58", "0.0.58")
	if !ok || newer {
		t.Fatalf("IsNewer same = %v, %v; want false, true", newer, ok)
	}
}

func TestReleaseAssetName(t *testing.T) {
	got := releaseAssetName("probakgo-windows-client")
	want := "probakgo-windows-client_" + runtime.GOOS + "_" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		want += ".exe"
	}
	if got != want {
		t.Fatalf("releaseAssetName = %q, want %q", got, want)
	}
}

func TestWindowsReplacementScriptRetriesAndDeletesItself(t *testing.T) {
	script := windowsReplacementScript(`C:\Probakgo\client.exe`, `C:\Probakgo\client.exe.new`)
	for _, want := range []string{
		`for /L %%i in (1,1,60)`,
		`move /Y "C:\Probakgo\client.exe.new" "C:\Probakgo\client.exe"`,
		`del "%~f0"`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("replacement script does not contain %q:\n%s", want, script)
		}
	}
}

func TestFetchLatestReleaseRetriesWithoutTokenWhenTokenRejected(t *testing.T) {
	oldBaseURL := githubAPIBaseURL
	t.Cleanup(func() { githubAPIBaseURL = oldBaseURL })
	t.Setenv("GITHUB_TOKEN", "expired-token")

	var authed, anonymous int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/releases/latest" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "" {
			authed++
			http.Error(w, "bad credentials", http.StatusUnauthorized)
			return
		}
		anonymous++
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"tag_name":"v1.2.3","assets":[]}`)
	}))
	defer srv.Close()
	githubAPIBaseURL = srv.URL

	rel, err := fetchLatestRelease("owner/repo")
	if err != nil {
		t.Fatalf("fetchLatestRelease: %v", err)
	}
	if rel.TagName != "v1.2.3" {
		t.Fatalf("tag: got %q, want v1.2.3", rel.TagName)
	}
	if authed != 1 || anonymous != 1 {
		t.Fatalf("requests: authed=%d anonymous=%d, want 1/1", authed, anonymous)
	}
}

func TestReleaseAssetRequestRetriesWithBrowserURLWhenTokenRejected(t *testing.T) {
	oldBaseURL := githubAPIBaseURL
	t.Cleanup(func() { githubAPIBaseURL = oldBaseURL })
	t.Setenv("GITHUB_TOKEN", "expired-token")

	var apiHits, browserHits int
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiHits++
		if r.URL.Path != "/repos/owner/repo/releases/assets/123" {
			t.Fatalf("unexpected asset path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") == "" {
			t.Fatal("expected authorization header on API asset request")
		}
		http.Error(w, "bad credentials", http.StatusForbidden)
	}))
	defer apiSrv.Close()
	browserSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		browserHits++
		if r.Header.Get("Authorization") != "" {
			t.Fatal("browser fallback should not include authorization")
		}
		_, _ = io.WriteString(w, "asset")
	}))
	defer browserSrv.Close()
	githubAPIBaseURL = apiSrv.URL

	resp, err := doWithGitHubAuthFallback(&http.Client{}, func(withAuth bool) (*http.Request, error) {
		return newReleaseAssetRequest("owner/repo", 123, browserSrv.URL+"/asset", withAuth)
	})
	if err != nil {
		t.Fatalf("request with fallback: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	if apiHits != 1 || browserHits != 1 {
		t.Fatalf("hits: api=%d browser=%d, want 1/1", apiHits, browserHits)
	}
}
