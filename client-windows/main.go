package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"probakgo/internal/selfupdate"
)

var version = "0.0.163"

func main() {
	closeLog := setupLogging()
	defer closeLog()

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version":
			fmt.Println(version)
			return
		case "install":
			if err := runInstall(os.Args[2:]); err != nil {
				log.Fatalf("install failed: %v", err)
			}
			return
		case "heartbeat":
			if err := runHeartbeat(); err != nil {
				log.Fatalf("heartbeat failed: %v", err)
			}
			return
		case "update":
			if err := runUpdate(); err != nil {
				log.Fatalf("update failed: %v", err)
			}
			return
		case "doctor":
			if err := runDoctor(); err != nil {
				log.Fatalf("doctor failed: %v", err)
			}
			return
		}
	}
	if err := runReport(); err != nil {
		log.Fatalf("report failed: %v", err)
	}
}

func runReport() error {
	cfg, err := loadConfig("")
	if err != nil {
		return err
	}
	req, err := buildReportRequest(context.Background(), cfg)
	if err != nil {
		return err
	}
	log.Printf("probakgo-windows-client v%s", version)
	log.Printf("Hostname    : %s", req.Hostname)
	log.Printf("API URL     : %s", cfg.APIURL)
	log.Printf("Disks       : %d", len(req.Disks))
	if err := postJSON(cfg, "/api/report/windows", req); err != nil {
		return err
	}
	log.Printf("Report sent successfully (%s)", time.Now().Format(time.RFC3339))
	return nil
}

func runHeartbeat() error {
	cfg, err := loadConfig("")
	if err != nil {
		return err
	}
	req, err := buildHeartbeatRequest(context.Background(), cfg)
	if err != nil {
		return err
	}
	return postJSON(cfg, "/api/heartbeat", req)
}

func runUpdate() error {
	if err := loadEnvIntoProcess(""); err != nil && !os.IsNotExist(err) {
		return err
	}
	_, err := selfupdate.Run("Nestorm18/probakgo", "probakgo-windows-client", version)
	return err
}

func runDoctor() error {
	cfg, err := loadConfig("")
	if err != nil {
		return err
	}
	if strings.TrimSpace(cfg.APIURL) == "" {
		return fmt.Errorf("API_URL is empty")
	}
	if err := validateAPIURL(cfg.APIURL); err != nil {
		return fmt.Errorf("API_URL is invalid: %w", err)
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return fmt.Errorf("API_KEY is empty")
	}
	ctx := context.Background()
	mid, err := machineID(ctx)
	if err != nil {
		return fmt.Errorf("machine id: %w", err)
	}
	disks, err := collectDisks(ctx)
	if err != nil {
		return fmt.Errorf("collect disks via PowerShell/WMI: %w", err)
	}
	hb, err := buildHeartbeatRequest(ctx, cfg)
	if err != nil {
		return fmt.Errorf("build heartbeat: %w", err)
	}
	if err := postJSON(cfg, "/api/heartbeat", hb); err != nil {
		return fmt.Errorf("Probakgo/API key check failed: %w", err)
	}
	fmt.Println("Configuration OK.")
	fmt.Println("Probakgo API: OK.")
	fmt.Printf("Machine ID: %s\n", mid)
	fmt.Printf("Disks detected: %d\n", len(disks))
	if err := checkScheduledTask(); err != nil {
		fmt.Printf("Scheduled task: WARN: %v\n", err)
	} else {
		fmt.Println("Scheduled task: OK.")
	}
	fmt.Println("Log:", logPath())
	return nil
}

func runInstall(args []string) error {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	apiURL := fs.String("api-url", "", "Probakgo server URL")
	apiKey := fs.String("api-key", "", "Probakgo API key")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*apiURL) == "" || strings.TrimSpace(*apiKey) == "" {
		return fmt.Errorf("--api-url and --api-key are required")
	}
	return installWindowsClient(Config{APIURL: normalizeAPIURL(*apiURL), APIKey: *apiKey})
}
