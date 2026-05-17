package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type discoveredVM struct {
	VMID string
	Name string
}

type backupConfigResponse struct {
	Configs []backupConfig `json:"configs"`
}

type backupConfig struct {
	VMID      string `json:"vm_id"`
	VMName    string `json:"vm_name"`
	Monday    bool   `json:"monday"`
	Tuesday   bool   `json:"tuesday"`
	Wednesday bool   `json:"wednesday"`
	Thursday  bool   `json:"thursday"`
	Friday    bool   `json:"friday"`
	Saturday  bool   `json:"saturday"`
	Sunday    bool   `json:"sunday"`
}

func (c *pveClient) discoverPVEVMs() ([]discoveredVM, error) {
	var vms []discoveredVM
	successes := 0
	for _, ep := range []string{
		fmt.Sprintf("nodes/%s/qemu", c.si.Hostname),
		fmt.Sprintf("nodes/%s/lxc", c.si.Hostname),
	} {
		data, err := c.get(ep)
		if err != nil {
			continue
		}
		successes++
		raw, _ := data["data"].([]any)
		for _, item := range raw {
			vm, ok := item.(map[string]any)
			if !ok {
				continue
			}
			vmidF, ok := vm["vmid"].(float64)
			if !ok {
				continue
			}
			name, _ := vm["name"].(string)
			vms = append(vms, discoveredVM{
				VMID: strconv.FormatInt(int64(vmidF), 10),
				Name: name,
			})
		}
	}
	if successes == 0 {
		return nil, fmt.Errorf("could not query QEMU or LXC inventory")
	}
	sort.Slice(vms, func(i, j int) bool {
		a, _ := strconv.Atoi(vms[i].VMID)
		b, _ := strconv.Atoi(vms[j].VMID)
		return a < b
	})
	return vms, nil
}

func syncBackupConfig(cfg *Config, serverName, machineID string, vms []discoveredVM) (created, updated, skipped int, err error) {
	if len(vms) == 0 {
		return 0, 0, 0, nil
	}
	apiURL := strings.TrimRight(cfg.APIURL, "/")
	if apiURL == "" || cfg.APIKey == "" {
		return 0, 0, 0, fmt.Errorf("API_URL and API_KEY are required")
	}

	base := apiURL + "/api/backup-config/pve/" + url.PathEscape(serverName)
	existing, err := fetchExistingVMConfigs(base, cfg.APIKey, machineID)
	if err != nil {
		return 0, 0, 0, err
	}

	for _, vm := range vms {
		if existingCfg, ok := existing[vm.VMID]; ok {
			if !hasAnyScheduledDay(existingCfg) {
				if err := updateVMConfig(base, cfg.APIKey, machineID, vm); err != nil {
					return created, updated, skipped, err
				}
				updated++
				continue
			}
			skipped++
			continue
		}
		if err := createVMConfig(base, cfg.APIKey, machineID, vm); err != nil {
			return created, updated, skipped, err
		}
		created++
	}
	return created, updated, skipped, nil
}

func fetchExistingVMConfigs(baseURL, apiKey, machineID string) (map[string]backupConfig, error) {
	req, err := http.NewRequest(http.MethodGet, baseURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if machineID != "" {
		req.Header.Set("X-Machine-ID", machineID)
	}

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("get backup config: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get backup config returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed backupConfigResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parse backup config: %w", err)
	}
	existing := make(map[string]backupConfig, len(parsed.Configs))
	for _, cfg := range parsed.Configs {
		existing[cfg.VMID] = cfg
	}
	return existing, nil
}

func createVMConfig(baseURL, apiKey, machineID string, vm discoveredVM) error {
	payload := weekdayVMConfigPayload(vm)
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, baseURL+"/vms", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	if machineID != "" {
		req.Header.Set("X-Machine-ID", machineID)
	}

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("create backup config for VM %s: %w", vm.VMID, err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("create backup config for VM %s returned %d: %s", vm.VMID, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

func updateVMConfig(baseURL, apiKey, machineID string, vm discoveredVM) error {
	payload := weekdayVMConfigPayload(vm)
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, baseURL+"/vms/"+url.PathEscape(vm.VMID), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	if machineID != "" {
		req.Header.Set("X-Machine-ID", machineID)
	}

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("update backup config for VM %s: %w", vm.VMID, err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("update backup config for VM %s returned %d: %s", vm.VMID, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

func weekdayVMConfigPayload(vm discoveredVM) map[string]any {
	return map[string]any{
		"vm_id":     vm.VMID,
		"vm_name":   vm.Name,
		"monday":    true,
		"tuesday":   true,
		"wednesday": true,
		"thursday":  true,
		"friday":    true,
	}
}

func hasAnyScheduledDay(cfg backupConfig) bool {
	return cfg.Monday || cfg.Tuesday || cfg.Wednesday || cfg.Thursday || cfg.Friday || cfg.Saturday || cfg.Sunday
}
