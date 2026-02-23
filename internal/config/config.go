package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Version is injected at build time by goreleaser.
// If not injected, it defaults to "dev".
var Version = "dev"

// Config represents the application configuration
// Uses flat YAML structure: days at top-level, groups as sections
type Config struct {
	Days   int               `yaml:"days"`
	Groups map[string]*Group `yaml:",inline"`
}

type Group struct {
	Repos []string `yaml:"repos"`
}

type Repo struct {
	Name   string `yaml:"name"`
	Path   string `yaml:"path,omitempty"`
	URL    string `yaml:"url,omitempty"`
	Branch string `yaml:"branch,omitempty"`
}

// parseRepoString parses a repository string and extracts name, path, URL, and optional branch
func parseRepoString(repoStr string) (Repo, error) {
	repoStr = expandTilde(repoStr)

	base := repoStr
	branch := ""
	if idx := strings.LastIndex(repoStr, "#"); idx != -1 {
		base = repoStr[:idx]
		branch = repoStr[idx+1:]
	}

	var repo Repo
	if isGitURL(base) {
		repo = parseGitURL(base)
	} else {
		name := extractNameFromPath(base)
		repo = Repo{
			Name: name,
			Path: base,
		}
	}

	repo.Branch = branch
	return repo, nil
}

// expandTilde expands ~ to the user's home directory
// getHomeDir is a variable to allow mocking in tests
var getHomeDir = os.UserHomeDir

func expandTilde(path string) string {
	if strings.HasPrefix(path, "~/") || path == "~" {
		home, err := getHomeDir()
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

// RemoveRepo removes a repository from the specified group.
// The repo can be identified by its full path/URL or by its short name.
// Returns the removed repository string and an error if not found.
func (c *Config) RemoveRepo(repoIdentifier, groupName string) (string, error) {
	if c.Groups == nil {
		return "", fmt.Errorf("no groups configured")
	}

	group, ok := c.Groups[groupName]
	if !ok {
		return "", fmt.Errorf("group '%s' not found", groupName)
	}

	// First, try to find by exact match (full path or URL)
	for i, existingRepo := range group.Repos {
		if existingRepo == repoIdentifier {
			removed := group.Repos[i]
			group.Repos = append(group.Repos[:i], group.Repos[i+1:]...)
			return removed, nil
		}
	}

	// If not found by exact match, try to find by short name or display name (name#branch)
	for i, existingRepo := range group.Repos {
		repo, err := parseRepoString(existingRepo)
		if err != nil {
			continue
		}

		displayName := repo.Name
		if repo.Branch != "" {
			displayName = fmt.Sprintf("%s#%s", repo.Name, repo.Branch)
		}

		if repo.Name == repoIdentifier || displayName == repoIdentifier {
			removed := group.Repos[i]
			group.Repos = append(group.Repos[:i], group.Repos[i+1:]...)
			return removed, nil
		}
	}

	return "", fmt.Errorf("repository '%s' not found in group '%s'", repoIdentifier, groupName)
}

// Save saves the configuration to the specified file path using YAML encoder
// Writes flat format: days at top-level, groups as groupname sections
func (c *Config) Save(configFile string) error {
	if configFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		configFile = filepath.Join(home, ".config", "repomon", "config.yaml")
	}

	// Create directory if it doesn't exist
	configDir := filepath.Dir(configFile)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	f, err := os.Create(configFile)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer f.Close()

	enc := yaml.NewEncoder(f)
	defer enc.Close()
	if err := enc.Encode(c); err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	slog.Debug("Configuration saved successfully", "file", configFile)
	return nil
}

// Load the configuration from the specified YAML file path
func Load(configFile string) (*Config, error) {
	if configFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		configFile = filepath.Join(home, ".config", "repomon", "config.yaml")
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
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	if cfg.Days == 0 {
		cfg.Days = 1
	}
	if cfg.Groups == nil {
		cfg.Groups = make(map[string]*Group)
	}

	slog.Debug("Configuration loaded successfully", "file", configFile, "groups", len(cfg.Groups))
	return &cfg, nil
}
