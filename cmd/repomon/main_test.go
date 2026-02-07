package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/plars/repomon/internal/config"
	"github.com/plars/repomon/internal/git"
)

// mockGitMonitor is a mock implementation of the GitMonitor interface.
type mockGitMonitor struct {
	results []git.RepoResult
	err     error
	days    int
}

func (m *mockGitMonitor) GetRecentCommits(ctx context.Context) ([]git.RepoResult, error) {
	return m.results, m.err
}

func (m *mockGitMonitor) SetDays(days int) {
	m.days = days
}

// mockFormatter is a mock implementation of the ReportFormatter interface.
type mockFormatter struct {
	output string
	err    error
}

func (m *mockFormatter) Format(results []git.RepoResult) (string, error) {
	return m.output, m.err
}

func TestExecuteList(t *testing.T) {
	tests := []struct {
		name           string
		cfg            *config.Config
		loadErr        error
		rootOpts       *rootOptions
		expectedOutput string
		expectedError  string
	}{
		{
			name: "List default group with local and remote repos",
			cfg: &config.Config{
				Groups: map[string]*config.Group{
					"default": {
						Repos: []string{"/path/to/repo1", "https://github.com/test/remote-repo"},
					},
				},
			},
			rootOpts:       &rootOptions{group: ""},
			expectedOutput: "Repositories for group 'default':\n  - repo1: /path/to/repo1\n  - remote-repo: https://github.com/test/remote-repo (remote)\n",
		},
		{
			name: "List specific group with no repos",
			cfg: &config.Config{
				Groups: map[string]*config.Group{
					"emptygroup": {Repos: []string{}},
				},
			},
			rootOpts:       &rootOptions{group: "emptygroup"},
			expectedOutput: "No repositories found for group 'emptygroup'.\n",
		},
		{
			name: "List non-existent group, fallback to default",
			cfg: &config.Config{
				Groups: map[string]*config.Group{
					"default": {Repos: []string{"/path/to/repo1"}},
				},
			},
			rootOpts:       &rootOptions{group: "nonexistent"},
			expectedOutput: "Repositories for group 'default':\n  - repo1: /path/to/repo1\n",
		},
		{
			name:    "Config file not found",
			loadErr: fmt.Errorf("config file not found: /path/does/not/exist/config.toml"),
			rootOpts: &rootOptions{
				configFile: "/path/does/not/exist/config.toml",
				group:      "default",
			},
			expectedError: "failed to load configuration: config file not found: /path/does/not/exist/config.toml",
		},
		{
			name: "Config file is empty (no default group)",
			cfg:  &config.Config{Groups: make(map[string]*config.Group)},
			rootOpts: &rootOptions{
				group: "default",
			},
			expectedError: "failed to get repositories: no default group found in configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outBuf := new(bytes.Buffer)
			errBuf := new(bytes.Buffer)
			runner := newDefaultRunner(outBuf, errBuf)

			runner.loadConfig = func(path string) (*config.Config, error) {
				if tt.loadErr != nil {
					return nil, tt.loadErr
				}
				return tt.cfg, nil
			}

			err := runner.executeList(nil, tt.rootOpts)

			if tt.expectedError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("Expected error containing %q, got %v", tt.expectedError, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if outBuf.String() != tt.expectedOutput {
					t.Errorf("Expected output:\n%q\nGot:\n%q", tt.expectedOutput, outBuf.String())
				}
			}
		})
	}
}

func TestExecuteRun(t *testing.T) {
	tests := []struct {
		name           string
		cfg            *config.Config
		loadErr        error
		monitorResults []git.RepoResult
		monitorErr     error
		formatOutput   string
		formatErr      error
		runOpts        *runOptions
		rootOpts       *rootOptions
		expectedOutput string
		expectedError  string
	}{
		{
			name: "Successful run",
			cfg: &config.Config{
				Defaults: config.Defaults{Days: 1},
				Groups: map[string]*config.Group{
					"default": {Repos: []string{"/path/to/repo"}},
				},
			},
			monitorResults: []git.RepoResult{
				{
					Repo: config.Repo{Name: "repo", Path: "/path/to/repo"},
					Commits: []git.Commit{
						{Message: "Initial commit", Author: "Test User"},
					},
				},
			},
			formatOutput:   "Repository Monitor Report\nrepo: Initial commit",
			runOpts:        &runOptions{days: 1},
			rootOpts:       &rootOptions{group: "default"},
			expectedOutput: "Repository Monitor Report\nrepo: Initial commit",
		},
		{
			name:    "Config load failure",
			loadErr: fmt.Errorf("file not found"),
			runOpts: &runOptions{days: 1},
			rootOpts: &rootOptions{
				configFile: "missing.toml",
				group:      "default",
			},
			expectedError: "failed to load configuration: file not found",
		},
		{
			name: "Monitor failure",
			cfg: &config.Config{
				Groups: map[string]*config.Group{
					"default": {Repos: []string{"/path/to/repo"}},
				},
			},
			monitorErr:    fmt.Errorf("git error"),
			runOpts:       &runOptions{days: 1},
			rootOpts:      &rootOptions{group: "default"},
			expectedError: "failed to get recent commits: git error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outBuf := new(bytes.Buffer)
			errBuf := new(bytes.Buffer)
			runner := newDefaultRunner(outBuf, errBuf)

			runner.loadConfig = func(path string) (*config.Config, error) {
				if tt.loadErr != nil {
					return nil, tt.loadErr
				}
				return tt.cfg, nil
			}
			runner.newGitMonitor = func(repos []config.Repo) GitMonitor {
				return &mockGitMonitor{results: tt.monitorResults, err: tt.monitorErr}
			}
			runner.newFormatter = func() ReportFormatter {
				return &mockFormatter{output: tt.formatOutput, err: tt.formatErr}
			}

			err := runner.executeRun(context.Background(), nil, tt.runOpts, tt.rootOpts)

			if tt.expectedError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("Expected error containing %q, got %v", tt.expectedError, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if outBuf.String() != tt.expectedOutput {
					t.Errorf("Expected output %q, got %q", tt.expectedOutput, outBuf.String())
				}
			}
		})
	}
}

// Keep an integration test to ensure everything works together
func TestIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo1")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatal(err)
	}

	// Simple git setup for integration test
	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoPath
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\nOutput: %s", args, err, out)
		}
	}

	runGit("init")
	if err := os.WriteFile(filepath.Join(repoPath, "file"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit("add", ".")
	runGit("commit", "-m", "first")

	cfgPath := filepath.Join(tmpDir, "config.toml")
	cfgContent := fmt.Sprintf(`
[groups.default]
repos = ["%s"]
`, repoPath)
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	runner := newDefaultRunner(outBuf, errBuf)

	rootOpts := &rootOptions{configFile: cfgPath}
	runOpts := &runOptions{days: 1}

	err := runner.executeRun(context.Background(), nil, runOpts, rootOpts)
	if err != nil {
		t.Fatalf("Integration test failed: %v", err)
	}

	if !strings.Contains(outBuf.String(), "Repository Monitor Report") {
		t.Errorf("Expected report header, got: %s", outBuf.String())
	}
	if !strings.Contains(outBuf.String(), "first") {
		t.Errorf("Expected commit message, got: %s", outBuf.String())
	}
}
