package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	// Create a temporary directory for test config files
	tempDir, err := os.MkdirTemp("", "repomon-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test case 1: Valid config file
	validConfig := `
[defaults]
days = 7

[[repos]]
name = "test-repo-1"
path = "/path/to/repo1"

[[repos]]
name = "test-repo-2"
path = "/path/to/repo2"
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

	if len(cfg.Repos) != 2 {
		t.Errorf("Expected 2 repos, got %d", len(cfg.Repos))
	}

	if cfg.Repos[0].Name != "test-repo-1" {
		t.Errorf("Expected repo name 'test-repo-1', got '%s'", cfg.Repos[0].Name)
	}

	// Test case 2: Config file without defaults (should use default days)
	noDefaultsConfig := `
[[repos]]
name = "test-repo"
path = "/path/to/repo"
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

	// Test case 3: Non-existent config file
	_, err = Load(filepath.Join(tempDir, "non-existent.toml"))
	if err == nil {
		t.Error("Expected error for non-existent config file")
	}

	// Test case 4: Invalid TOML
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
	// Test loading with empty config path (should use default)
	// This will likely fail unless the user has a config file, which is expected
	cfg, err := Load("")
	if err == nil {
		// If it succeeds, verify it's a valid config
		if cfg.Defaults.Days <= 0 {
			t.Error("Default days should be positive")
		}
	} else {
		// Expected to fail in most test environments
		t.Logf("Expected failure for default config path: %v", err)
	}
}