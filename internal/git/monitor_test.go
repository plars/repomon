package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/plars/repomon/internal/config"
)

func TestMonitor_GetRecentCommits(t *testing.T) {
	// Create a temporary directory for test repositories
	tempDir, err := os.MkdirTemp("", "repomon-git-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test git repository
	repoPath := filepath.Join(tempDir, "test-repo")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}

	// Initialize git repo
	if err := initTestRepo(repoPath); err != nil {
		t.Fatalf("Failed to initialize test repo: %v", err)
	}

	cfg := &config.Config{
		Defaults: config.Defaults{Days: 7},
		Repos:    []string{repoPath},
	}

	monitor := NewMonitor(cfg)
	results, err := monitor.GetRecentCommits(context.Background())
	if err != nil {
		t.Fatalf("Failed to get recent commits: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	result := results[0]
	if result.Repo.Name != "test-repo" {
		t.Errorf("Expected repo name 'test-repo', got '%s'", result.Repo.Name)
	}

	if result.Error != nil {
		t.Errorf("Unexpected error: %v", result.Error)
	}
}

func TestMonitor_getRepoCommits(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "repomon-git-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	repoPath := filepath.Join(tempDir, "test-repo")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}

	cfg := &config.Config{
		Defaults: config.Defaults{Days: 7},
	}

	monitor := NewMonitor(cfg)
	repo := config.Repo{Name: "test-repo", Path: repoPath}

	// Test with non-existent path
	_, err = monitor.getRepoCommits(context.Background(), repo)
	if err == nil {
		t.Error("Expected error for non-existent path")
	}

	// Initialize git repo and test with valid path
	if err := initTestRepo(repoPath); err != nil {
		t.Fatalf("Failed to initialize test repo: %v", err)
	}

	commits, err := monitor.getRepoCommits(context.Background(), repo)
	if err != nil {
		t.Fatalf("Failed to get commits from valid repo: %v", err)
	}

	// Should have at least one commit (the initial commit)
	if len(commits) == 0 {
		t.Error("Expected at least one commit")
	}
}

func TestMonitor_getRepoCommits_NotGitRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "repomon-git-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create directory but don't initialize git repo
	repoPath := filepath.Join(tempDir, "not-git-repo")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}

	cfg := &config.Config{
		Defaults: config.Defaults{Days: 7},
	}

	monitor := NewMonitor(cfg)
	repo := config.Repo{Name: "not-git-repo", Path: repoPath}

	_, err = monitor.getRepoCommits(context.Background(), repo)
	if err == nil {
		t.Error("Expected error for non-git repository")
	}
}

func TestMonitor_parseGitLog(t *testing.T) {
	cfg := &config.Config{
		Defaults: config.Defaults{Days: 7},
	}

	monitor := NewMonitor(cfg)

	// Test that parseGitLog is deprecated
	_, err := monitor.parseGitLog([]byte(""))
	if err == nil {
		t.Error("Expected error for deprecated parseGitLog function")
	}

	expectedError := "parseGitLog deprecated with go-git implementation"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

// Helper function to initialize a test git repository using go-git
func initTestRepo(repoPath string) error {
	return initGitRepo(repoPath)
}

func initGitRepo(repoPath string) error {
	// Create a simple test file to ensure we have content
	testFile := filepath.Join(repoPath, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		return err
	}

	// Use go-git to initialize repository
	_, err := git.PlainInit(repoPath, false)
	if err != nil {
		return err
	}

	// Open the repository and create initial commit
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return err
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return err
	}

	// Add the test file
	_, err = worktree.Add("test.txt")
	if err != nil {
		return err
	}

	// Create commit
	_, err = worktree.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})

	return err
}
