package config

import (
	"os"
	"path/filepath"
)

// Config represents application configuration settings.
type Config struct {
	DBPath           string  `json:"db_path" yaml:"db_path"`
	CardsPath        string  `json:"cards_path" yaml:"cards_path"`
	RequestRetention float64 `json:"request_retention" yaml:"request_retention"`
	MaximumInterval  float64 `json:"maximum_interval" yaml:"maximum_interval"`
	Theme            string  `json:"theme" yaml:"theme"`
}

// DefaultConfig returns default configuration values.
func DefaultConfig() *Config {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}

	dbDir := filepath.Join(home, ".local", "share", "rota")
	dbPath := filepath.Join(dbDir, "rota.db")

	return &Config{
		DBPath:           dbPath,
		CardsPath:        ".",
		RequestRetention: 0.90,
		MaximumInterval:  36500,
		Theme:            "dark",
	}
}

// ResolveDBPath resolves the database path from flag, env, or default.
func ResolveDBPath(flagVal string) string {
	if flagVal != "" {
		return flagVal
	}
	if env := os.Getenv("ROTA_DB"); env != "" {
		return env
	}

	// Check if local .rota/rota.db exists in current workspace
	if _, err := os.Stat(".rota/rota.db"); err == nil {
		return ".rota/rota.db"
	}

	return DefaultConfig().DBPath
}
