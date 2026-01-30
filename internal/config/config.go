package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"log/slog"
)

// Config represents the application configuration
type Config struct {
	Defaults Defaults `toml:"defaults"`
	Repos    []string `toml:"repos"`
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

// parseRepoString parses a repository string and extracts name, path, and URL
func parseRepoString(repoStr string) (Repo, error) {
	repoStr = expandTilde(repoStr)

	if isGitURL(repoStr) {
		return parseGitURL(repoStr), nil
	}

	name := extractNameFromPath(repoStr)
	return Repo{
		Name: name,
		Path: repoStr,
	}, nil
}

// expandTilde expands ~ to the user's home directory
func expandTilde(path string) string {
	if strings.HasPrefix(path, "~/") || path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			slog.Warn("Failed to get home directory for ~ expansion", "error", err)
			return path
		}
		if path == "~" {
			return home
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

// isGitURL checks if a string is a Git URL
func isGitURL(s string) bool {
	if strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "http://") {
		return true
	}

	if strings.HasPrefix(s, "git@") {
		return true
	}

	if strings.HasPrefix(s, "git://") {
		return true
	}

	if strings.HasPrefix(s, "ssh://") {
		return true
	}

	return false
}

// parseGitURL parses a Git URL and extracts name and URL
func parseGitURL(urlStr string) Repo {
	cleanURL := strings.TrimSuffix(urlStr, ".git")

	name := ""

	if strings.HasPrefix(cleanURL, "git@") {
		parts := strings.SplitN(cleanURL, ":", 2)
		if len(parts) == 2 {
			name = extractNameFromPath(parts[1])
		} else {
			name = "unknown"
		}
		return Repo{
			Name: name,
			URL:  urlStr,
		}
	}

	if u, err := url.Parse(cleanURL); err == nil {
		name = extractNameFromPath(u.Path)
		return Repo{
			Name: name,
			URL:  urlStr,
		}
	}

	name = extractNameFromPath(cleanURL)
	return Repo{
		Name: name,
		URL:  urlStr,
	}
}

// extractNameFromPath extracts the final directory name from a path
func extractNameFromPath(path string) string {
	cleanPath := filepath.Clean(path)
	base := filepath.Base(cleanPath)

	if base == "" || base == "." || base == "/" {
		return "unknown"
	}

	return base
}

// GetRepos parses and returns the repositories as Repo structs
func (c *Config) GetRepos() []Repo {
	repos := make([]Repo, 0, len(c.Repos))

	for _, repoStr := range c.Repos {
		repo, err := parseRepoString(repoStr)
		if err != nil {
			slog.Warn("Failed to parse repository string", "string", repoStr, "error", err)
			continue
		}
		repos = append(repos, repo)
	}

	return repos
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
