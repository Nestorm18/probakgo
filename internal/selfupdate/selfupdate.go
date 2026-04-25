package selfupdate

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type githubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// Run checks GitHub for a newer release of binaryName and self-updates if one exists.
// repo is "owner/repo", e.g. "Nestorm18/probaky".
// currentVersion is the running binary's version (e.g. "dev" or "v1.2.0").
func Run(repo, binaryName, currentVersion string) error {
	if currentVersion == "dev" {
		fmt.Println("Dev build - skipping version check")
		return nil
	}

	fmt.Printf("Checking for updates (%s %s)...\n", binaryName, currentVersion)

	tag, downloadURL, err := latestRelease(repo, binaryName)
	if err != nil {
		return fmt.Errorf("check release: %w", err)
	}

	if tag == currentVersion || strings.TrimPrefix(tag, "v") == strings.TrimPrefix(currentVersion, "v") {
		fmt.Printf("Already up to date (%s)\n", currentVersion)
		return nil
	}

	fmt.Printf("New version: %s → %s\n", currentVersion, tag)
	fmt.Println("Downloading...")

	if err := replace(downloadURL); err != nil {
		return fmt.Errorf("update: %w", err)
	}

	fmt.Printf("Updated to %s. Restart to apply.\n", tag)
	return nil
}

// LatestTag returns the tag of the latest GitHub release (for display purposes).
func LatestTag(repo string) (string, error) {
	rel, err := fetchLatestRelease(repo)
	if err != nil {
		return "", err
	}
	return rel.TagName, nil
}

func latestRelease(repo, binaryName string) (tag, url string, err error) {
	rel, err := fetchLatestRelease(repo)
	if err != nil {
		return "", "", err
	}
	assetName := fmt.Sprintf("%s_%s_%s", binaryName, runtime.GOOS, runtime.GOARCH)
	for _, a := range rel.Assets {
		if a.Name == assetName {
			return rel.TagName, a.BrowserDownloadURL, nil
		}
	}
	return rel.TagName, "", fmt.Errorf("asset %q not found in release %s", assetName, rel.TagName)
}

func fetchLatestRelease(repo string) (*githubRelease, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get("https://api.github.com/repos/" + repo + "/releases/latest")
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

func replace(downloadURL string) error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate executable: %w", err)
	}
	// Resolve symlinks so we replace the real file
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return fmt.Errorf("resolve symlinks: %w", err)
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	// Write to a temp file in the same directory for atomic rename
	tmpPath := executable + ".new"
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write: %w", err)
	}
	f.Close()

	// Atomic replace - on Linux the kernel keeps the old inode open, so this is safe
	if err := os.Rename(tmpPath, executable); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("replace binary: %w", err)
	}
	return nil
}
