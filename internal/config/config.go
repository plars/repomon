package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
	"log/slog"
)

// Config represents the application configuration
type Config struct {
	Defaults Defaults `toml:"defaults"`
	Repos    []Repo   `toml:"repos"`
}

// Defaults contains default configuration values
type Defaults struct {
	Days int `toml:"days"`
}

// Repo represents a repository configuration
type Repo struct {
	Name string `toml:"name"`
	Path string `toml:"path,omitempty"`
	URL  string `toml:"url,omitempty"`
}

// Load loads the configuration from the specified file path
func Load(configFile string) (*Config, error) {
	if configFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		configFile = filepath.Join(home, ".config", "repomon", "config.toml")
	}

	// Check if config file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", configFile)
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse TOML: %w", err)
	}

	// Set default values if not specified
	if cfg.Defaults.Days == 0 {
		cfg.Defaults.Days = 1
	}

	slog.Debug("Configuration loaded successfully", "file", configFile, "repos", len(cfg.Repos))
	return &cfg, nil
}
