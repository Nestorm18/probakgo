package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DBPath         string
	APIHost        string
	APIPort        string
	SessionKey     string
	Timezone       string
	SecureSession  bool
	TrustedOrigins []string
	Dev            bool
}

func Load() *Config {
	return &Config{
		DBPath:         getEnv("DATABASE_PATH", "probakgo_data.db"),
		APIHost:        getEnv("API_HOST", "0.0.0.0"),
		APIPort:        getEnv("API_PORT", "36748"),
		SessionKey:     loadSessionKey(),
		Timezone:       getEnv("TIMEZONE", "Europe/Madrid"),
		SecureSession:  getEnv("SESSION_SECURE", "false") == "true",
		TrustedOrigins: parseTrustedOrigins(os.Getenv("CSRF_TRUSTED_ORIGINS")),
		Dev:            os.Getenv("DEV") == "true",
	}
}

// parseTrustedOrigins parses CSRF_TRUSTED_ORIGINS (comma-separated host:port values).
// gorilla/csrf compares against the Host portion of the Origin header, so the
// format must be "host:port" (e.g. "probakgo.local:36748"), not a full URL.
func parseTrustedOrigins(raw string) []string {
	if raw == "" {
		return nil
	}
	var out []string
	for _, s := range strings.Split(raw, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func loadSessionKey() string {
	if key := os.Getenv("SESSION_KEY"); key != "" {
		return key
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("config: cannot generate random SESSION_KEY: " + err.Error())
	}
	slog.Warn("SESSION_KEY not set - using random key, all sessions will be lost on restart. Set SESSION_KEY in .env for persistent sessions.")
	return hex.EncodeToString(b)
}

func (c *Config) Validate() error {
	if _, err := time.LoadLocation(c.Timezone); err != nil {
		return fmt.Errorf("invalid TIMEZONE %q: %w", c.Timezone, err)
	}
	port, err := strconv.Atoi(c.APIPort)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid API_PORT %q: must be an integer between 1 and 65535", c.APIPort)
	}
	if len(c.SessionKey) < 32 {
		return fmt.Errorf("SESSION_KEY is too short (%d bytes): minimum 32 bytes required", len(c.SessionKey))
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func GetInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
