package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func postJSON(cfg Config, path string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	req, err := httpRequest(ctx, "POST", apiURL(cfg.APIURL, path), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	if mid := machineIDFromPayload(payload); mid != "" {
		req.Header.Set("X-Machine-ID", mid)
	}
	resp, err := httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("server returned %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
}

func machineIDFromPayload(payload any) string {
	switch v := payload.(type) {
	case interface{ GetMachineID() string }:
		return v.GetMachineID()
	case map[string]any:
		if s, ok := v["machine_id"].(string); ok {
			return s
		}
	default:
		b, err := json.Marshal(payload)
		if err != nil {
			return ""
		}
		var tmp struct {
			MachineID string `json:"machine_id"`
		}
		if json.Unmarshal(b, &tmp) == nil {
			return tmp.MachineID
		}
	}
	return ""
}

func apiURL(base, path string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if strings.HasSuffix(base, "/api") {
		return base + strings.TrimPrefix(path, "/api")
	}
	return base + path
}

func httpRequest(ctx context.Context, method, url string, body io.Reader) (*http.Request, error) {
	return http.NewRequestWithContext(ctx, method, url, body)
}

func httpClient() *http.Client {
	return &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		},
	}
}
