package config

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"os"
	"strconv"
)

type Config struct {
	DBPath     string
	APIHost    string
	APIPort    string
	SessionKey string
	Timezone   string
}

func Load() *Config {
	return &Config{
		DBPath:     getEnv("DATABASE_PATH", "probakgo_data.db"),
		APIHost:    getEnv("API_HOST", "0.0.0.0"),
		APIPort:    getEnv("API_PORT", "36748"),
		SessionKey: loadSessionKey(),
		Timezone:   getEnv("TIMEZONE", "Europe/Madrid"),
	}
}

func loadSessionKey() string {
	if key := os.Getenv("SESSION_KEY"); key != "" {
		return key
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("config: cannot generate random SESSION_KEY: " + err.Error())
	}
	slog.Warn("SESSION_KEY not set — using random key, all sessions will be lost on restart. Set SESSION_KEY in .env for persistent sessions.")
	return hex.EncodeToString(b)
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
