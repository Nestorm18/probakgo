package main

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	APIURL string
	APIKey string
}

func configPath() string {
	if p := os.Getenv("PROBAKGO_WINDOWS_ENV"); p != "" {
		return p
	}
	base := os.Getenv("ProgramData")
	if base == "" {
		base = `C:\ProgramData`
	}
	return filepath.Join(base, "Probakgo", ".env")
}

func installDir() string {
	base := os.Getenv("ProgramData")
	if base == "" {
		base = `C:\ProgramData`
	}
	return filepath.Join(base, "Probakgo")
}

func logPath() string {
	return filepath.Join(installDir(), "probakgo-windows-client.log")
}

func loadConfig(path string) (Config, error) {
	if path == "" {
		path = configPath()
	}
	f, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	cfg := Config{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		val = strings.Trim(strings.TrimSpace(val), `"`)
		switch strings.TrimSpace(key) {
		case "API_URL":
			cfg.APIURL = val
		case "API_KEY":
			cfg.APIKey = val
		}
	}
	if err := sc.Err(); err != nil {
		return Config{}, err
	}
	cfg.APIURL = normalizeAPIURL(cfg.APIURL)
	return cfg, nil
}

func normalizeAPIURL(raw string) string {
	s := strings.TrimRight(strings.TrimSpace(raw), "/")
	if s == "" || strings.Contains(s, "://") {
		return s
	}
	return "http://" + s
}

func validateAPIURL(raw string) error {
	u, err := url.Parse(normalizeAPIURL(raw))
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("scheme must be http or https")
	}
	if u.Host == "" {
		return fmt.Errorf("host is empty")
	}
	return nil
}
