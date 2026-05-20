package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"

	"probakgo/internal/selfupdate"
)

var version = "0.0.42"

func main() {
	log.SetFlags(log.Ldate | log.Ltime)

	for _, p := range []string{"/opt/probakgo/.env", ".env"} {
		if _, err := os.Stat(p); err == nil {
			_ = godotenv.Load(p)
			break
		}
	}

	// Subcommands handled before flag.Parse so they get their own flag sets.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "install":
			runInstall(os.Args[2:])
			return
		case "uninstall":
			runUninstall(os.Args[2:])
			return
		case "update":
			if err := selfupdate.Run("Nestorm18/probakgo", "probakgo-client", version); err != nil {
				fmt.Fprintf(os.Stderr, "Update failed: %v\n", err)
				os.Exit(1)
			}
			return
		case "version":
			fmt.Printf("probakgo-client v%s\n", version)
			return
		}
	}

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
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: probakgo-client [subcommand] [flags]\n\n")
		fmt.Fprintf(os.Stderr, "Subcommands:\n")
		fmt.Fprintf(os.Stderr, "  install     Install on this Proxmox node and configure the vzdump hook\n")
		fmt.Fprintf(os.Stderr, "  uninstall   Remove all installed files and revoke the Proxmox API token\n")
		fmt.Fprintf(os.Stderr, "  update      Self-update to the latest GitHub release\n")
		fmt.Fprintf(os.Stderr, "  version     Print version\n\n")
		fmt.Fprintf(os.Stderr, "Flags (report mode):\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	log.Printf("probakgo-client v%s", version)

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

	if vzdumpHook || fromFile != "" || cfg.ServerType == "pbs" {
		if err := sendReport(cfg, si, fromFile); err != nil {
			log.Printf("ERROR: %v", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	fmt.Println("Configuration OK.")
	fmt.Println("Use --vzdump-hook to send a report after each backup.")
}
