package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

func sendReport(cfg *Config, si *SysInfo, fromFile string) error {
	var (
		data    map[string]any
		urlPath string
		err     error
	)

	if fromFile != "" {
		raw, err := os.ReadFile(fromFile)
		if err != nil {
			return fmt.Errorf("read %s: %w", fromFile, err)
		}
		if err = json.Unmarshal(raw, &data); err != nil {
			return fmt.Errorf("parse file: %w", err)
		}
		urlPath = "report/pve"
		if cfg.ServerType == "pbs" {
			urlPath = "report/pbs"
		}
	} else {
		switch cfg.ServerType {
		case "pve":
			data, err = newPVEClient(cfg, si).generateReport()
			urlPath = "report/pve"
		case "pbs":
			data, err = newPBSClient(cfg, si).generateReport()
			urlPath = "report/pbs"
		default:
			return fmt.Errorf("unknown server type: %s", cfg.ServerType)
		}
		if err != nil {
			return fmt.Errorf("generate report: %w", err)
		}
	}

	body, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}

	url := cfg.APIURL + "/" + urlPath
	log.Printf("Sending report to %s ...", url)

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("connection error: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 200:
		log.Printf("Report sent successfully (%s)", time.Now().Format(time.RFC3339))
		return nil
	case 401:
		return fmt.Errorf("authentication error: API key invalid or inactive")
	default:
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
}
