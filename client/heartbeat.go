package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

func sendHeartbeat(cfg *Config, si *SysInfo) error {
	machineID := si.machineID()
	data := map[string]any{
		"hostname":       si.Hostname,
		"server_type":    cfg.ServerType,
		"ip_address":     si.localIP(),
		"client_version": version,
		"machine_id":     machineID,
	}
	body, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal heartbeat: %w", err)
	}
	url := cfg.APIURL + "/api/heartbeat"
	log.Printf("Sending heartbeat to %s ...", url)
	if cfg.Debug {
		log.Printf("DEBUG heartbeat payload: %s", string(body))
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	if machineID != "" {
		req.Header.Set("X-Machine-ID", machineID)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("connection error: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	switch resp.StatusCode {
	case http.StatusOK:
		log.Printf("Heartbeat sent successfully (%s)", time.Now().Format(time.RFC3339))
		return nil
	case http.StatusUnauthorized:
		return fmt.Errorf("authentication error: API key invalid or inactive")
	default:
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(respBody))
	}
}
