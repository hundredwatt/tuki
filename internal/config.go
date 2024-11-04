package internal

import (
	"fmt"
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
	TickIntervalSeconds      int
	MaxTicks                 int
	Verbose                  bool
	Mode                     Mode
}

func LoadConfig() *Config {
	return &Config{
		RepoURL:                  getEnvOrFatal("REPO_URL"),
		ScriptsDir:               getEnvOrDefault("SCRIPTS_DIR", "/"),
		InProgressTimeoutMinutes: getEnvAsIntOrDefault("IN_PROGRESS_TIMEOUT_MINUTES", 60),
		StateFile:                getEnvOrDefault("STATE_FILE", ".tuki/state.jsonl"),
		TickIntervalSeconds:      getEnvAsIntOrDefault("TICK_INTERVAL_SECONDS", 60),
		MaxTicks:                 getEnvAsIntOrDefault("MAX_TICKS", -1),
		Verbose:                  getEnvAsBoolOrDefault("VERBOSE", false),
		Mode:                     getModeFromEnv("MODE", Periodic),
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

func getModeFromEnv(key string, defaultValue Mode) Mode {
	value := strings.ToLower(os.Getenv(key))
	switch value {
	case "":
		return defaultValue
	case strings.ToLower(string(Manual)):
		return Manual
	case strings.ToLower(string(Periodic)):
		return Periodic
	default:
		panic(fmt.Sprintf("Invalid mode: %s", value))
	}
}
