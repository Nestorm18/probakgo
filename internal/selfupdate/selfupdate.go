package selfupdate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var githubAPIBaseURL = "https://api.github.com"

type githubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		ID                 int64  `json:"id"`
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// Run checks GitHub for a newer release of binaryName and self-updates if one exists.
// It returns true only when the binary was replaced.
// repo is "owner/repo", e.g. "Nestorm18/probakgo".
// currentVersion is the running binary's version (e.g. "dev" or "v1.2.0").
func Run(repo, binaryName, currentVersion string) (bool, error) {
	if currentVersion == "dev" {
		fmt.Println("Dev build - skipping version check")
		return false, nil
	}

	fmt.Printf("Checking for updates (%s %s)...\n", binaryName, currentVersion)

	tag, binID, sha256ID, binaryURL, sha256URL, err := latestRelease(repo, binaryName)
	if err != nil {
		return false, fmt.Errorf("check release: %w", err)
	}

	cmp, ok := compareVersions(tag, currentVersion)
	if !ok {
		fmt.Printf("Cannot compare versions (%s vs %s) - skipping update\n", tag, currentVersion)
		return false, nil
	}
	if cmp <= 0 {
		fmt.Printf("No newer version available (local %s, remote %s)\n", currentVersion, tag)
		return false, nil
	}

	if tag == currentVersion || strings.TrimPrefix(tag, "v") == strings.TrimPrefix(currentVersion, "v") {
		fmt.Printf("Already up to date (%s)\n", currentVersion)
		return false, nil
	}

	fmt.Printf("New version: %s → %s\n", currentVersion, tag)
	fmt.Println("Downloading...")

	if err := replace(repo, binID, sha256ID, binaryURL, sha256URL, binaryName); err != nil {
		return false, fmt.Errorf("update: %w", err)
	}

	fmt.Printf("Updated to %s. Restart to apply.\n", tag)
	return true, nil
}

// IsNewer reports whether remote is a newer semantic version than current.
func IsNewer(remote, current string) (bool, bool) {
	cmp, ok := compareVersions(remote, current)
	return cmp > 0, ok
}

// LatestTag returns the tag of the latest GitHub release (for display purposes).
func LatestTag(repo string) (string, error) {
	rel, err := fetchLatestRelease(repo)
	if err != nil {
		return "", err
	}
	return rel.TagName, nil
}

func latestRelease(repo, binaryName string) (tag string, binID, sha256ID int64, binaryURL, sha256URL string, err error) {
	rel, err := fetchLatestRelease(repo)
	if err != nil {
		return "", 0, 0, "", "", err
	}
	assetName := releaseAssetName(binaryName)
	for _, a := range rel.Assets {
		switch a.Name {
		case assetName:
			binaryURL = a.BrowserDownloadURL
			binID = a.ID
		case "SHA256SUMS":
			sha256URL = a.BrowserDownloadURL
			sha256ID = a.ID
		}
	}
	if binaryURL == "" {
		return rel.TagName, 0, 0, "", "", fmt.Errorf("asset %q not found in release %s", assetName, rel.TagName)
	}
	return rel.TagName, binID, sha256ID, binaryURL, sha256URL, nil
}

func compareVersions(remote, current string) (int, bool) {
	remoteParts, ok := versionParts(remote)
	if !ok {
		return 0, false
	}
	currentParts, ok := versionParts(current)
	if !ok {
		return 0, false
	}
	maxLen := len(remoteParts)
	if len(currentParts) > maxLen {
		maxLen = len(currentParts)
	}
	for i := 0; i < maxLen; i++ {
		var r, c int
		if i < len(remoteParts) {
			r = remoteParts[i]
		}
		if i < len(currentParts) {
			c = currentParts[i]
		}
		if r > c {
			return 1, true
		}
		if r < c {
			return -1, true
		}
	}
	return 0, true
}

func versionParts(v string) ([]int, bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	if v == "" {
		return nil, false
	}
	raw := strings.Split(v, ".")
	parts := make([]int, 0, len(raw))
	for _, part := range raw {
		if part == "" {
			return nil, false
		}
		n := 0
		for _, ch := range part {
			if ch < '0' || ch > '9' {
				return nil, false
			}
			n = n*10 + int(ch-'0')
		}
		parts = append(parts, n)
	}
	return parts, true
}

func githubToken() string {
	return strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
}

func newAPIRequest(method, url string) (*http.Request, error) {
	return newGitHubRequest(method, url, true)
}

func newGitHubRequest(method, url string, withAuth bool) (*http.Request, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	if tok := githubToken(); withAuth && tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	return req, nil
}

func fetchLatestRelease(repo string) (*githubRelease, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := doWithGitHubAuthFallback(client, func(withAuth bool) (*http.Request, error) {
		return newGitHubRequest("GET", githubAPIURL("/repos/"+repo+"/releases/latest"), withAuth)
	})
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}
	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return &rel, nil
}

func githubAPIURL(path string) string {
	return strings.TrimRight(githubAPIBaseURL, "/") + path
}

func doWithGitHubAuthFallback(client *http.Client, build func(withAuth bool) (*http.Request, error)) (*http.Response, error) {
	resp, err := doBuiltRequest(client, build, true)
	if err != nil {
		return nil, err
	}
	if githubToken() == "" || !shouldRetryGitHubAnonymous(resp.StatusCode) {
		return resp, nil
	}
	status := resp.StatusCode
	resp.Body.Close()
	fmt.Printf("GitHub token rejected (HTTP %d), retrying without token...\n", status)
	return doBuiltRequest(client, build, false)
}

func doBuiltRequest(client *http.Client, build func(withAuth bool) (*http.Request, error), withAuth bool) (*http.Response, error) {
	req, err := build(withAuth)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

func shouldRetryGitHubAnonymous(status int) bool {
	return status == http.StatusUnauthorized || status == http.StatusForbidden
}

func replace(repo string, binID, sha256ID int64, downloadURL, sha256URL, binaryName string) error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate executable: %w", err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return fmt.Errorf("resolve symlinks: %w", err)
	}

	client := &http.Client{Timeout: 5 * time.Minute}

	resp, err := doWithGitHubAuthFallback(client, func(withAuth bool) (*http.Request, error) {
		return newReleaseAssetRequest(repo, binID, downloadURL, withAuth)
	})
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}

	tmpPath := executable + ".new"
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), resp.Body); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write: %w", err)
	}
	f.Close()
	actualHash := hex.EncodeToString(h.Sum(nil))

	if sha256URL == "" || sha256ID == 0 {
		os.Remove(tmpPath)
		return fmt.Errorf("release does not include SHA256SUMS")
	}
	if err := verifyChecksum(client, repo, sha256ID, sha256URL, binaryName, actualHash); err != nil {
		os.Remove(tmpPath)
		return err
	}

	if runtime.GOOS == "windows" {
		return replaceWindowsExecutable(executable, tmpPath)
	}

	// Atomic replace - on Linux the kernel keeps the old inode open, so this is safe
	if err := os.Rename(tmpPath, executable); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("replace binary: %w", err)
	}
	return nil
}

func verifyChecksum(client *http.Client, repo string, sha256ID int64, sha256URL, binaryName, actualHash string) error {
	resp, err := doWithGitHubAuthFallback(client, func(withAuth bool) (*http.Request, error) {
		return newReleaseAssetRequest(repo, sha256ID, sha256URL, withAuth)
	})
	if err != nil {
		return fmt.Errorf("fetch checksums: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("checksum download returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read checksums: %w", err)
	}
	assetName := releaseAssetName(binaryName)
	for _, line := range strings.Split(string(body), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == assetName {
			if parts[0] != actualHash {
				return fmt.Errorf("checksum mismatch for %s", assetName)
			}
			return nil
		}
	}
	return fmt.Errorf("checksum not found in SHA256SUMS for %s", assetName)
}

func newReleaseAssetRequest(repo string, assetID int64, browserURL string, withAuth bool) (*http.Request, error) {
	if githubToken() != "" && withAuth && assetID != 0 {
		apiURL := fmt.Sprintf("%s/repos/%s/releases/assets/%d", strings.TrimRight(githubAPIBaseURL, "/"), repo, assetID)
		req, err := newGitHubRequest("GET", apiURL, true)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/octet-stream")
		return req, nil
	}
	return http.NewRequest("GET", browserURL, nil)
}

func releaseAssetName(binaryName string) string {
	name := fmt.Sprintf("%s_%s_%s", binaryName, runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

func replaceWindowsExecutable(executable, tmpPath string) error {
	scriptPath := tmpPath + ".cmd"
	script := fmt.Sprintf(`@echo off
ping 127.0.0.1 -n 3 > nul
move /Y "%s" "%s" > nul
del "%%~f0" > nul 2>&1
`, tmpPath, executable)
	if err := os.WriteFile(scriptPath, []byte(script), 0600); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("create replacement script: %w", err)
	}
	cmd := exec.Command("cmd", "/C", "start", "", "/B", scriptPath)
	if err := cmd.Start(); err != nil {
		os.Remove(tmpPath)
		os.Remove(scriptPath)
		return fmt.Errorf("start replacement script: %w", err)
	}
	return nil
}
