package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
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
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("PBS API %s returned HTTP %d: %s", endpoint, resp.StatusCode, strings.TrimSpace(string(body)))
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
	if list, ok := data["data"].([]any); ok {
		for _, item := range list {
			ds, ok := item.(map[string]any)
			if !ok {
				continue
			}
			storeName, _ := ds["store"].(string)
			if storeName == "" {
				continue
			}
			groups, err := c.get(fmt.Sprintf("admin/datastore/%s/groups", storeName))
			if err != nil {
				log.Printf("WARN: PBS groups for %q: %v", storeName, err)
				continue
			}
			groupList, _ := groups["data"].([]any)

			// Fetch snapshot sizes and verification states (groups endpoint omits both).
			// Build map (backup-type/backup-id/backup-time) → {size, verification-state}.
			type snapInfo struct {
				size       int64
				verifState string
			}
			snapData := map[string]snapInfo{}
			if snaps, err := c.get(fmt.Sprintf("admin/datastore/%s/snapshots", storeName)); err == nil {
				for _, s := range snaps["data"].([]any) {
					snap, ok := s.(map[string]any)
					if !ok {
						continue
					}
					bt, _ := snap["backup-type"].(string)
					bi, _ := snap["backup-id"].(string)
					ts, _ := snap["backup-time"].(float64)
					sz, _ := snap["size"].(float64)
					var verifState string
					if v, ok := snap["verification"].(map[string]any); ok {
						verifState, _ = v["state"].(string)
					}
					key := fmt.Sprintf("%s/%s/%d", bt, bi, int64(ts))
					snapData[key] = snapInfo{size: int64(sz), verifState: verifState}
				}
			} else {
				log.Printf("WARN: PBS snapshots for %q: %v", storeName, err)
			}

			// Inject size and verification-state of the latest snapshot into each group.
			for _, g := range groupList {
				grp, ok := g.(map[string]any)
				if !ok {
					continue
				}
				bt, _ := grp["backup-type"].(string)
				bi, _ := grp["backup-id"].(string)
				lastBackup, _ := grp["last-backup"].(float64)
				key := fmt.Sprintf("%s/%s/%d", bt, bi, int64(lastBackup))
				if info, found := snapData[key]; found {
					grp["size"] = info.size
					grp["verification-state"] = info.verifState
				}
			}
			ds["groups"] = groupList
		}
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
