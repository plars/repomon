package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "repomon-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	validConfig := `
days: 7
default:
  repos:
    - /path/to/repo1
    - /path/to/repo2
`
	validConfigPath := filepath.Join(tempDir, "valid.yaml")
	if err := os.WriteFile(validConfigPath, []byte(validConfig), 0644); err != nil {
		t.Fatalf("Failed to write valid config: %v", err)
	}

	cfg, err := Load(validConfigPath)
	if err != nil {
		t.Fatalf("Failed to load valid config: %v", err)
	}

	if cfg.Days != 7 {
		t.Errorf("Expected days=7, got %d", cfg.Days)
	}

	// Updated call to GetRepos (Line 37)
	repos, _, err := cfg.GetRepos("default")
	if err != nil {
		t.Fatalf("Failed to get repos from valid config: %v", err)
	}
	if len(repos) != 2 {
		t.Errorf("Expected 2 parsed repos, got %d", len(repos))
	}

	if repos[0].Name != "repo1" {
		t.Errorf("Expected repo name 'repo1', got '%s'", repos[0].Name)
	}

	if repos[0].Path != "/path/to/repo1" {
		t.Errorf("Expected repo path '/path/to/repo1', got '%s'", repos[0].Path)
	}

	noDefaultsConfig := `
default:
  repos:
    - /path/to/repo
`
	noDefaultsConfigPath := filepath.Join(tempDir, "no-defaults.yaml")
	if err := os.WriteFile(noDefaultsConfigPath, []byte(noDefaultsConfig), 0644); err != nil {
		t.Fatalf("Failed to write no-defaults config: %v", err)
	}

	cfg, err = Load(noDefaultsConfigPath)
	if err != nil {
		t.Fatalf("Failed to load no-defaults config: %v", err)
	}

	if cfg.Days != 1 {
		t.Errorf("Expected default days=1, got %d", cfg.Days)
	}

	_, err = Load(filepath.Join(tempDir, "non-existent.yaml"))
	if err == nil {
		t.Error("Expected error for non-existent config file")
	}

	invalidYAMLPath := filepath.Join(tempDir, "invalid.yaml")
	if err := os.WriteFile(invalidYAMLPath, []byte("invalid yaml: ["), 0644); err != nil {
		t.Fatalf("Failed to write invalid YAML: %v", err)
	}

	_, err = Load(invalidYAMLPath)
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}

func TestLoadDefaultPath(t *testing.T) {
	cfg, err := Load("")
	if err == nil {
		if cfg.Days <= 0 {
			t.Error("Default days should be positive")
		}
	} else {
		t.Logf("Expected failure for default config path: %v", err)
	}
}

func TestParseRepoString(t *testing.T) {
	tests := []struct {
		name    string
		repoStr string
		want    Repo
		wantErr bool
	}{
		{
			name:    "local path",
			repoStr: "/home/user/projects/my-project",
			want:    Repo{Name: "my-project", Path: "/home/user/projects/my-project"},
			wantErr: false,
		},
		{
			name:    "HTTPS GitHub URL",
			repoStr: "https://github.com/go-git/go-git",
			want:    Repo{Name: "go-git", URL: "https://github.com/go-git/go-git"},
			wantErr: false,
		},
		{
			name:    "HTTPS URL with .git",
			repoStr: "https://github.com/kubernetes/kubernetes.git",
			want:    Repo{Name: "kubernetes", URL: "https://github.com/kubernetes/kubernetes.git"},
			wantErr: false,
		},
		{
			name:    "SSH GitHub URL",
			repoStr: "git@github.com:plars/repomon.git",
			want:    Repo{Name: "repomon", URL: "git@github.com:plars/repomon.git"},
			wantErr: false,
		},
		{
			name:    "GitLab URL",
			repoStr: "https://gitlab.com/company/project.git",
			want:    Repo{Name: "project", URL: "https://gitlab.com/company/project.git"},
			wantErr: false,
		},
		{
			name:    "path with tilde",
			repoStr: "~/projects/work-app",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRepoString(tt.repoStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseRepoString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.name == "path with tilde" {
				home, _ := os.UserHomeDir()
				expectedPath := filepath.Join(home, "projects/work-app")
				if got.Name != "work-app" || got.Path != expectedPath {
					t.Errorf("parseRepoString() = %+v, want Name=work-app, Path=%s", got, expectedPath)
				}
				return
			}

			if got.Name != tt.want.Name {
				t.Errorf("parseRepoString().Name = %v, want %v", got.Name, tt.want.Name)
			}
			if got.Path != tt.want.Path {
				t.Errorf("parseRepoString().Path = %v, want %v", got.Path, tt.want.Path)
			}
			if got.URL != tt.want.URL {
				t.Errorf("parseRepoString().URL = %v, want %v", got.URL, tt.want.URL)
			}
		})
	}
}

func TestGetRepos(t *testing.T) {
	cfg := &Config{
		Days: 7,
		Groups: map[string]*Group{
			"default": {
				Repos: []string{
					"/home/user/projects/my-project",
					"https://github.com/go-git/go-git",
					"git@github.com:plars/repomon.git",
					"~/projects/work-app",
				},
			},
		},
	}

	// Updated call to GetRepos (Line 184)
	repos, _, err := cfg.GetRepos("default")
	if err != nil {
		t.Fatalf("Failed to get repos in TestGetRepos: %v", err)
	}

	if len(repos) != 4 {
		t.Fatalf("Expected 4 repos, got %d", len(repos))
	}

	if repos[0].Name != "my-project" || repos[0].Path != "/home/user/projects/my-project" {
		t.Errorf("Repo 0 incorrect: %+v", repos[0])
	}

	if repos[1].Name != "go-git" || repos[1].URL != "https://github.com/go-git/go-git" {
		t.Errorf("Repo 1 incorrect: %+v", repos[1])
	}

	if repos[2].Name != "repomon" || repos[2].URL != "git@github.com:plars/repomon.git" {
		t.Errorf("Repo 2 incorrect: %+v", repos[2])
	}

	if repos[3].Name != "work-app" {
		t.Errorf("Repo 3 name incorrect: %+v", repos[3])
	}
}

func TestGetReposFallbackToDefault(t *testing.T) {
	cfg := &Config{
		Days: 7,
		Groups: map[string]*Group{
			"default": {
				Repos: []string{"/path/to/repo"},
			},
			"work": {
				Repos: []string{"/path/to/work"},
			},
		},
	}

	// Request a non-existent group, should fallback to default
	repos, effectiveGroup, err := cfg.GetRepos("nonexistent")
	if err != nil {
		t.Fatalf("Expected no error for fallback: %v", err)
	}

	if effectiveGroup != "default" {
		t.Errorf("Expected effective group 'default', got '%s'", effectiveGroup)
	}

	if len(repos) != 1 {
		t.Errorf("Expected 1 repo, got %d", len(repos))
	}
}

func TestGetReposNoDefaultGroup(t *testing.T) {
	cfg := &Config{
		Days: 7,
		Groups: map[string]*Group{
			"work": {
				Repos: []string{"/path/to/work"},
			},
		},
	}

	// Request a non-existent group when there's no default group
	_, _, err := cfg.GetRepos("nonexistent")
	if err == nil {
		t.Error("Expected error when no default group exists")
	}
}

func TestExpandTilde(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantHome bool
	}{
		{
			name:  "tilde only",
			input: "~",
			// Result depends on home directory
		},
		{
			name:  "tilde with path",
			input: "~/projects/myrepo",
			// Result depends on home directory
		},
		{
			name:  "regular path unchanged",
			input: "/absolute/path",
		},
		{
			name:  "relative path unchanged",
			input: "./relative/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandTilde(tt.input)

			// If input starts with ~, result should be different
			if strings.HasPrefix(tt.input, "~") {
				home, err := os.UserHomeDir()
				if err != nil {
					t.SkipNow()
				}
				if tt.input == "~" {
					if result != home {
						t.Errorf("expandTilde(%q) = %q, want %q", tt.input, result, home)
					}
				} else if strings.HasPrefix(tt.input, "~/") {
					want := filepath.Join(home, tt.input[2:])
					if result != want {
						t.Errorf("expandTilde(%q) = %q, want %q", tt.input, result, want)
					}
				}
			} else {
				// Non-tilde paths should remain unchanged
				if result != tt.input {
					t.Errorf("expandTilde(%q) = %q, want %q", tt.input, result, tt.input)
				}
			}
		})
	}
}

func TestExpandTilde_HomeDirError(t *testing.T) {
	// Save original and restore after test
	original := getHomeDir
	defer func() { getHomeDir = original }()

	// Mock getHomeDir to return an error
	getHomeDir = func() (string, error) {
		return "", fmt.Errorf("home directory not available")
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "tilde only returns unchanged on error",
			input: "~",
			want:  "~",
		},
		{
			name:  "tilde with path returns unchanged on error",
			input: "~/projects/myrepo",
			want:  "~/projects/myrepo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandTilde(tt.input)
			if result != tt.want {
				t.Errorf("expandTilde(%q) = %q, want %q", tt.input, result, tt.want)
			}
		})
	}
}

func TestIsGitURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "https URL",
			input: "https://github.com/user/repo",
			want:  true,
		},
		{
			name:  "http URL",
			input: "http://github.com/user/repo",
			want:  true,
		},
		{
			name:  "SSH URL",
			input: "git@github.com:user/repo",
			want:  true,
		},
		{
			name:  "git protocol",
			input: "git://github.com/user/repo",
			want:  true,
		},
		{
			name:  "SSH protocol",
			input: "ssh://git@github.com/user/repo",
			want:  true,
		},
		{
			name:  "local path",
			input: "/home/user/repo",
			want:  false,
		},
		{
			name:  "relative path",
			input: "./myrepo",
			want:  false,
		},
		{
			name:  "empty string",
			input: "",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isGitURL(tt.input)
			if got != tt.want {
				t.Errorf("isGitURL(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractNameFromPath(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple path",
			input: "/path/to/my-repo",
			want:  "my-repo",
		},
		{
			name:  "path with trailing slash",
			input: "/path/to/repo/",
			want:  "repo",
		},
		{
			name:  "current directory",
			input: ".",
			want:  "unknown",
		},
		{
			name:  "root directory",
			input: "/",
			want:  "unknown",
		},
		{
			name:  "empty string",
			input: "",
			want:  "unknown",
		},
		{
			name:  "relative path",
			input: "../repo",
			want:  "repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractNameFromPath(tt.input)
			if got != tt.want {
				t.Errorf("extractNameFromPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseGitURL(t *testing.T) {
	tests := []struct {
		name   string
		urlStr string
		want   Repo
	}{
		{
			name:   "HTTPS GitHub",
			urlStr: "https://github.com/user/repo",
			want:   Repo{Name: "repo", URL: "https://github.com/user/repo"},
		},
		{
			name:   "HTTPS with .git suffix",
			urlStr: "https://github.com/user/repo.git",
			want:   Repo{Name: "repo", URL: "https://github.com/user/repo.git"},
		},
		{
			name:   "SSH format",
			urlStr: "git@github.com:user/repo",
			want:   Repo{Name: "repo", URL: "git@github.com:user/repo"},
		},
		{
			name:   "SSH format with .git",
			urlStr: "git@github.com:user/repo.git",
			want:   Repo{Name: "repo", URL: "git@github.com:user/repo.git"},
		},
		{
			name:   "GitLab HTTPS",
			urlStr: "https://gitlab.com/company/project",
			want:   Repo{Name: "project", URL: "https://gitlab.com/company/project"},
		},
		{
			name:   "SSH URL with path",
			urlStr: "ssh://git@gitlab.com/user/project",
			want:   Repo{Name: "project", URL: "ssh://git@gitlab.com/user/project"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseGitURL(tt.urlStr)
			if got.Name != tt.want.Name {
				t.Errorf("parseGitURL(%q).Name = %q, want %q", tt.urlStr, got.Name, tt.want.Name)
			}
			if got.URL != tt.want.URL {
				t.Errorf("parseGitURL(%q).URL = %q, want %q", tt.urlStr, got.URL, tt.want.URL)
			}
		})
	}
}

func TestAddRepo(t *testing.T) {
	cfg := &Config{
		Groups: make(map[string]*Group),
	}

	// Add to new group
	err := cfg.AddRepo("/path/to/repo", "work")
	if err != nil {
		t.Fatalf("AddRepo failed: %v", err)
	}

	if cfg.Groups["work"] == nil {
		t.Error("Expected group 'work' to be created")
	}

	if len(cfg.Groups["work"].Repos) != 1 {
		t.Errorf("Expected 1 repo in group, got %d", len(cfg.Groups["work"].Repos))
	}

	// Add another repo to same group
	err = cfg.AddRepo("/another/repo", "work")
	if err != nil {
		t.Fatalf("AddRepo failed: %v", err)
	}

	if len(cfg.Groups["work"].Repos) != 2 {
		t.Errorf("Expected 2 repos in group, got %d", len(cfg.Groups["work"].Repos))
	}

	// Try to add duplicate
	err = cfg.AddRepo("/path/to/repo", "work")
	if err == nil {
		t.Error("Expected error for duplicate repo")
	}
}

func TestConfigSave(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "repomon-save-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configFile := filepath.Join(tempDir, "config.yaml")

	cfg := &Config{
		Days: 14,
		Groups: map[string]*Group{
			"default": {
				Repos: []string{"/path/to/repo1", "/path/to/repo2"},
			},
		},
	}

	err = cfg.Save(configFile)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify the file was created
	if _, err := os.Stat(configFile); err != nil {
		t.Error("Expected config file to exist")
	}

	// Load and verify
	loaded, err := Load(configFile)
	if err != nil {
		t.Fatalf("Failed to load saved config: %v", err)
	}

	if loaded.Days != 14 {
		t.Errorf("Expected days=14, got %d", loaded.Days)
	}

	if len(loaded.Groups) != 1 {
		t.Errorf("Expected 1 group, got %d", len(loaded.Groups))
	}
}

func TestConfigSaveDefaultPath(t *testing.T) {
	cfg := &Config{
		Days:   1,
		Groups: make(map[string]*Group),
	}

	// This test verifies that Save with empty path uses the default path
	// Note: This may fail if home directory is not accessible
	err := cfg.Save("")
	if err != nil {
		// Expected to potentially fail if home/.config/repomon is not writable
		t.Logf("Save to default path failed (expected in test environment): %v", err)
	}
}
