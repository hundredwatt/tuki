package internal

import (
	"log"
	"os"
	"strconv"
)

type Config struct {
	RepoURL                  string
	InProgressTimeoutMinutes int
	StateFile                string
	TickIntervalSeconds     int
	MaxTicks                int
}

func LoadConfig() *Config {
	return &Config{
		RepoURL:                  getEnvOrFatal("REPO_URL"),
		InProgressTimeoutMinutes: getEnvAsIntOrDefault("IN_PROGRESS_TIMEOUT_MINUTES", 60),
		StateFile:                getEnvOrDefault("STATE_FILE", ".tuki/state.jsonl"),
		TickIntervalSeconds:     getEnvAsIntOrDefault("TICK_INTERVAL_SECONDS", 60),
		MaxTicks:                getEnvAsIntOrDefault("MAX_TICKS", -1),
	}
}

func getEnvOrFatal(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("Environment variable %s is not set", key)
	}
	return value
}

func getEnvAsIntOrDefault(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	intValue, err := strconv.Atoi(value)
	if err != nil {
		log.Fatalf("Environment variable %s is not a valid integer: %v", key, err)
	}
	return intValue
}

func getEnvOrDefault(key string, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
