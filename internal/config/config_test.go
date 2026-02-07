package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "repomon-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	validConfig := `
[groups.default]
repos = ["/path/to/repo1", "/path/to/repo2"]

[defaults]
days = 7
`
	validConfigPath := filepath.Join(tempDir, "valid.toml")
	if err := os.WriteFile(validConfigPath, []byte(validConfig), 0644); err != nil {
		t.Fatalf("Failed to write valid config: %v", err)
	}

	cfg, err := Load(validConfigPath)
	if err != nil {
		t.Fatalf("Failed to load valid config: %v", err)
	}

	if cfg.Defaults.Days != 7 {
		t.Errorf("Expected days=7, got %d", cfg.Defaults.Days)
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
[groups.default]
repos = ["/path/to/repo"]
`
	noDefaultsConfigPath := filepath.Join(tempDir, "no-defaults.toml")
	if err := os.WriteFile(noDefaultsConfigPath, []byte(noDefaultsConfig), 0644); err != nil {
		t.Fatalf("Failed to write no-defaults config: %v", err)
	}

	cfg, err = Load(noDefaultsConfigPath)
	if err != nil {
		t.Fatalf("Failed to load no-defaults config: %v", err)
	}

	if cfg.Defaults.Days != 1 {
		t.Errorf("Expected default days=1, got %d", cfg.Defaults.Days)
	}

	_, err = Load(filepath.Join(tempDir, "non-existent.toml"))
	if err == nil {
		t.Error("Expected error for non-existent config file")
	}

	invalidTOMLPath := filepath.Join(tempDir, "invalid.toml")
	if err := os.WriteFile(invalidTOMLPath, []byte("invalid toml ["), 0644); err != nil {
		t.Fatalf("Failed to write invalid TOML: %v", err)
	}

	_, err = Load(invalidTOMLPath)
	if err == nil {
		t.Error("Expected error for invalid TOML")
	}
}

func TestLoadDefaultPath(t *testing.T) {
	cfg, err := Load("")
	if err == nil {
		if cfg.Defaults.Days <= 0 {
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
		Defaults: Defaults{Days: 7},
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