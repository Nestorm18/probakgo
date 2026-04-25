package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

type pbsClient struct {
	cfg     *Config
	si      *SysInfo
	http    *http.Client
	baseURL string
}

func newPBSClient(cfg *Config, si *SysInfo) *pbsClient {
	return &pbsClient{
		cfg:     cfg,
		si:      si,
		http:    newHTTPClient(cfg),
		baseURL: fmt.Sprintf("https://%s:8007/api2/json", si.localIP()),
	}
}

func (c *pbsClient) get(endpoint string) (map[string]any, error) {
	url := c.baseURL + "/" + endpoint
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	// PBS uses colon separator: PBSAPIToken=user:tokenid:secret
	req.Header.Set("Authorization", fmt.Sprintf("PBSAPIToken=%s:%s", c.cfg.ProxmoxToken, c.cfg.ProxmoxSecret))

	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			log.Printf("WARN: PBS %s attempt %d/3: %v", endpoint, attempt, err)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == 401 {
			return nil, fmt.Errorf("PBS auth error (401): check PROXMOX_TOKEN and PROXMOX_SECRET")
		}
		var result map[string]any
		if err := json.Unmarshal(body, &result); err != nil {
			lastErr = fmt.Errorf("invalid JSON: %w", err)
			log.Printf("WARN: PBS %s attempt %d/3: %v", endpoint, attempt, lastErr)
			continue
		}
		return result, nil
	}
	return nil, fmt.Errorf("PBS API %s failed after 3 attempts: %w", endpoint, lastErr)
}

func (c *pbsClient) validateConnection() bool {
	_, err := c.get("version")
	if err != nil {
		log.Printf("ERROR: %v", err)
	}
	return err == nil
}

func (c *pbsClient) generateReport() (map[string]any, error) {
	data, err := c.get("status/datastore-usage")
	if err != nil {
		return nil, fmt.Errorf("datastore-usage: %w", err)
	}
	return map[string]any{
		"hostname":        c.si.Hostname,
		"ip_address":      c.si.localIP(),
		"public_ip":       c.si.publicIP(),
		"client_version":  version,
		"machine_id":      c.si.machineID(),
		"pbs_information": data,
	}, nil
}
