package config

import (
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
		SessionKey: getEnv("SESSION_KEY", "change-me-in-production-32bytes!"),
		Timezone:   getEnv("TIMEZONE", "Europe/Madrid"),
	}
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
