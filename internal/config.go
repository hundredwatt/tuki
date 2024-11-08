package internal

import (
	"log"
	"os"
	"strconv"
	"strings"
)

type Mode string

const (
	Periodic Mode = "Periodic"
	Manual   Mode = "Manual"
)

type Config struct {
	RepoURL                  string
	ScriptsDir               string
	InProgressTimeoutMinutes int
	StateFile                string
	HarnessFile              string
	TickIntervalSeconds      int
	MaxTicks                 int
	Verbose                  bool
}

func LoadConfig() *Config {
	return &Config{
		RepoURL:                  getEnvOrFatal("REPO_URL"),
		ScriptsDir:               getEnvOrDefault("SCRIPTS_DIR", "/"),
		InProgressTimeoutMinutes: getEnvAsIntOrDefault("IN_PROGRESS_TIMEOUT_MINUTES", 60),
		StateFile:                getEnvOrDefault("STATE_FILE", ".tuki/state.jsonl"),
		HarnessFile:              getEnvOrDefault("HARNESS_FILE", ".tuki/harness.sh"),
		TickIntervalSeconds:      getEnvAsIntOrDefault("TICK_INTERVAL_SECONDS", 60),
		MaxTicks:                 getEnvAsIntOrDefault("MAX_TICKS", -1),
		Verbose:                  getEnvAsBoolOrDefault("VERBOSE", false),
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

func getEnvAsBoolOrDefault(key string, defaultValue bool) bool {
	value := strings.ToLower(os.Getenv(key))
	if value == "" {
		return defaultValue
	}
	return value == "true" || value == "t" || value == "1"
}
