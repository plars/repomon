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
	Defaults Defaults          `toml:"defaults"`
	Groups   map[string]*Group `toml:"groups"`
}

type Group struct {
	Repos []string `toml:"repos"`
}

type Defaults struct {
	Days int `toml:"days"`
}

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

// GetRepos retrieves the list of repositories for the given group name.
// If the group name is not found, it falls back to the "default" group.
// It returns the list of repositories, the effective group name used, and an error if the group (or fallback default) is not found.
func (c *Config) GetRepos(requestedGroupName string) ([]Repo, string, error) { // Added error return
	effectiveGroupName := requestedGroupName
	group, ok := c.Groups[requestedGroupName]
	if !ok {
		slog.Warn("Group not found, using 'default' group", "requested", requestedGroupName)
		effectiveGroupName = "default"
		group = c.Groups["default"]
		if group == nil {
			slog.Error("No default group found in configuration")
			return []Repo{}, effectiveGroupName, fmt.Errorf("no default group found in configuration") // Explicitly return error
		}
	}

	repos := make([]Repo, 0, len(group.Repos))
	for _, repoStr := range group.Repos {
		repo, err := parseRepoString(repoStr)
		if err != nil {
			slog.Warn("Failed to parse repository string", "string", repoStr, "error", err)
			continue
		}
		repos = append(repos, repo)
	}

	return repos, effectiveGroupName, nil // Return nil error on success
}

// AddRepo adds a repository to the specified group
func (c *Config) AddRepo(repoStr, groupName string) error {
	if c.Groups == nil {
		c.Groups = make(map[string]*Group)
	}

	group, ok := c.Groups[groupName]
	if !ok {
		group = &Group{
			Repos: []string{},
		}
		c.Groups[groupName] = group
	}

	// Check if repo already exists
	for _, existingRepo := range group.Repos {
		if existingRepo == repoStr {
			return fmt.Errorf("repository '%s' already exists in group '%s'", repoStr, groupName)
		}
	}

	group.Repos = append(group.Repos, repoStr)
	return nil
}

// formatTOML formats the config with proper indentation and spacing
func (c *Config) formatTOML() ([]byte, error) {
	var builder strings.Builder

	// Write defaults section
	if c.Defaults.Days != 0 {
		builder.WriteString("[defaults]\n")
		builder.WriteString(fmt.Sprintf("days = %d\n\n", c.Defaults.Days))
	}

	// Write each group section directly
	if c.Groups != nil && len(c.Groups) > 0 {
		// Write groups in alphabetical order for consistency
		groupNames := make([]string, 0, len(c.Groups))
		for groupName := range c.Groups {
			groupNames = append(groupNames, groupName)
		}

		for i, groupName := range groupNames {
			group := c.Groups[groupName]
			if group != nil && len(group.Repos) > 0 {
				builder.WriteString(fmt.Sprintf("[groups.%s]\n", groupName))
				builder.WriteString("repos = [\n")

				for _, repo := range group.Repos {
					builder.WriteString(fmt.Sprintf("    \"%s\",\n", repo))
				}

				builder.WriteString("]\n")

				// Add blank line between groups (but not after the last one)
				if i < len(groupNames)-1 {
					builder.WriteString("\n")
				}
			}
		}
	}

	return []byte(builder.String()), nil
}

// Save saves the configuration to the specified file path with proper formatting
func (c *Config) Save(configFile string) error {
	if configFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		configFile = filepath.Join(home, ".config", "repomon", "config.toml")
	}

	// Create directory if it doesn't exist
	configDir := filepath.Dir(configFile)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := c.formatTOML()
	if err != nil {
		return fmt.Errorf("failed to format TOML: %w", err)
	}

	if err := os.WriteFile(configFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	slog.Debug("Configuration saved successfully", "file", configFile)
	return nil
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

	slog.Debug("Configuration loaded successfully", "file", configFile, "groups", len(cfg.Groups))
	return &cfg, nil
}
