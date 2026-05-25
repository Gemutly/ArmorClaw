package chatrelay

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration for the chat relay subsystem.
type Config struct {
	Enabled     bool
	RoomIDs     []string
	MaxTokens   int
	Model       string        // empty = use existing ArmorClaw AI default
	Timeout     time.Duration
	MaxInFlight int           // bounded concurrent goroutines for AI calls
}

// ConfigFromEnv reads chat relay configuration from environment variables.
func ConfigFromEnv() *Config {
	cfg := &Config{
		Enabled:     parseBoolEnv("ARMORCLAW_CHAT_RELAY_ENABLED", false),
		MaxTokens:   parseIntEnv("ARMORCLAW_CHAT_RELAY_MAX_TOKENS", 256),
		Model:       os.Getenv("ARMORCLAW_CHAT_RELAY_MODEL"),
		Timeout:     parseDurationEnv("ARMORCLAW_CHAT_RELAY_TIMEOUT", 30*time.Second),
		MaxInFlight: parseIntEnv("ARMORCLAW_CHAT_RELAY_MAX_IN_FLIGHT", 4),
	}

	cfg.RoomIDs = parseRoomIDs(os.Getenv("ARMORCLAW_CHAT_RELAY_ROOM_IDS"))

	if cfg.Enabled && len(cfg.RoomIDs) == 0 {
		slog.Warn("chat relay enabled but no rooms configured, disabling")
		cfg.Enabled = false
	}

	return cfg
}

// IsRoomEnabled checks whether the given room ID is in the allowlist.
func (c *Config) IsRoomEnabled(roomID string) bool {
	for _, id := range c.RoomIDs {
		if id == roomID {
			return true
		}
	}
	return false
}

// parseBoolEnv reads an env var as a boolean. Accepts "true", "1", "yes" as truthy.
func parseBoolEnv(key string, fallback bool) bool {
	val := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if val == "" {
		return fallback
	}
	return val == "true" || val == "1" || val == "yes"
}

// parseIntEnv reads an env var as an integer with a fallback default.
func parseIntEnv(key string, fallback int) int {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		slog.Warn("invalid integer env var, using default", "key", key, "value", val, "default", fallback)
		return fallback
	}
	return n
}

// parseDurationEnv reads an env var as a Go duration string with a fallback default.
func parseDurationEnv(key string, fallback time.Duration) time.Duration {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback
	}
	d, err := time.ParseDuration(val)
	if err != nil {
		slog.Warn("invalid duration env var, using default", "key", key, "value", val, "default", fallback)
		return fallback
	}
	return d
}

// parseRoomIDs splits a comma-separated list of Matrix room IDs, trimming whitespace.
func parseRoomIDs(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	ids := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			ids = append(ids, p)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	return ids
}
