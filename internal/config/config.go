package config

import (
	"os"
	"strconv"
)

type Config struct {
	DBPath      string
	APIHost     string
	APIPort     string
	WebHost     string
	WebPort     string
	SessionKey  string
	Timezone    string
	AdminAPIKey string // printed on first run only
}

func Load() *Config {
	return &Config{
		DBPath:      getEnv("DATABASE_PATH", "probaky_data.db"),
		APIHost:     getEnv("API_HOST", "0.0.0.0"),
		APIPort:     getEnv("API_PORT", "36748"),
		WebHost:     getEnv("WEB_HOST", "0.0.0.0"),
		WebPort:     getEnv("WEB_PORT", "36749"),
		SessionKey:  getEnv("SESSION_KEY", "change-me-in-production-32bytes!"),
		Timezone:    getEnv("TIMEZONE", "Europe/Madrid"),
		AdminAPIKey: getEnv("ADMIN_API_KEY", ""),
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
