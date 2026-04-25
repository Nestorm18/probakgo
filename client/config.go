package main

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	APIURL        string
	APIKey        string
	ProxmoxToken  string
	ProxmoxSecret string
	VerifyTLS     bool
	CABundle      string
	Debug         bool
	DebugAPICalls bool
	ServerType    string
}

func loadConfig() *Config {
	for _, p := range []string{"/opt/probaky/.env", ".env"} {
		if _, err := os.Stat(p); err == nil {
			_ = godotenv.Load(p)
			break
		}
	}
	return &Config{
		APIURL:        os.Getenv("API_URL"),
		APIKey:        os.Getenv("API_KEY"),
		ProxmoxToken:  os.Getenv("PROXMOX_TOKEN"),
		ProxmoxSecret: os.Getenv("PROXMOX_SECRET"),
		VerifyTLS:     strings.ToLower(os.Getenv("PROXMOX_VERIFY_TLS")) != "false",
		CABundle:      strings.TrimSpace(os.Getenv("PROXMOX_CA_BUNDLE")),
		Debug:         strings.ToLower(os.Getenv("DEBUG_MODE")) == "true",
		DebugAPICalls: strings.ToLower(os.Getenv("DEBUG_API_CALLS")) == "true",
		ServerType:    os.Getenv("SERVER_TYPE"),
	}
}
