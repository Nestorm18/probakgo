package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

const version = "1.0.0"

func main() {
	var (
		debug      bool
		debugAPI   bool
		serverType string
		vzdumpHook bool
		fromFile   string
	)

	flag.BoolVar(&debug, "debug", false, "Enable verbose debug logging")
	flag.BoolVar(&debugAPI, "debug-api-calls", false, "Save raw API responses to debug/")
	flag.StringVar(&serverType, "server-type", "", "Force server type: pve or pbs")
	flag.BoolVar(&vzdumpHook, "vzdump-hook", false, "Send report immediately (called by vzdump hook)")
	flag.StringVar(&fromFile, "file", "", "Send report from a JSON file (for testing)")
	flag.Parse()

	log.SetFlags(log.Ldate | log.Ltime)
	log.Printf("probaky-client v%s", version)

	cfg := loadConfig()
	if debug {
		cfg.Debug = true
	}
	if debugAPI {
		cfg.DebugAPICalls = true
	}
	if serverType != "" {
		cfg.ServerType = serverType
	}

	si := newSysInfo(cfg)

	if cfg.ServerType == "" || cfg.ServerType == "unknown" {
		cfg.ServerType = si.detectServerType()
	}
	if cfg.ServerType == "unknown" {
		log.Println("ERROR: could not detect server type (PVE or PBS)")
		log.Println("Hint: check /etc/issue or use --server-type pve|pbs")
		os.Exit(1)
	}

	log.Printf("Server type : %s", cfg.ServerType)
	log.Printf("Hostname    : %s", si.Hostname)

	if !strings.HasPrefix(cfg.APIKey, "pbk-") {
		log.Println("ERROR: API key must start with 'pbk-'")
		os.Exit(1)
	}
	if cfg.APIURL == "" {
		log.Println("ERROR: API_URL not configured")
		os.Exit(1)
	}
	log.Printf("API URL     : %s", cfg.APIURL)

	switch cfg.ServerType {
	case "pve":
		if !newPVEClient(cfg, si).validateConnection() {
			log.Println("ERROR: Proxmox VE credentials invalid or unreachable")
			os.Exit(1)
		}
	case "pbs":
		if !newPBSClient(cfg, si).validateConnection() {
			log.Println("ERROR: Proxmox Backup Server credentials invalid or unreachable")
			os.Exit(1)
		}
	}
	log.Println("Proxmox connection OK")

	if vzdumpHook || fromFile != "" {
		if err := sendReport(cfg, si, fromFile); err != nil {
			log.Printf("ERROR: %v", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	fmt.Println("Configuration OK.")
	fmt.Println("Use --vzdump-hook to send a report after each backup.")
}
