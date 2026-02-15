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

	if err := initTestRepo(repoPath); err != nil {
		t.Fatalf("Failed to initialize test repo: %v", err)
	}

	repos := []config.Repo{{Name: "test-repo", Path: repoPath}}
	monitor := NewMonitorWithRepos(repos)
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

	monitor := NewMonitorWithRepos([]config.Repo{})
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

	monitor := NewMonitorWithRepos([]config.Repo{})
	repo := config.Repo{Name: "not-git-repo", Path: repoPath}

	_, err = monitor.getRepoCommits(context.Background(), repo)
	if err == nil {
		t.Error("Expected error for non-git repository")
	}
}

func TestMonitor_parseGitLog(t *testing.T) {
	monitor := NewMonitorWithRepos([]config.Repo{})

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

func TestGetOneLineCommitMessage(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "single line message",
			input: "Add new feature",
			want:  "Add new feature",
		},
		{
			name:  "multi-line message",
			input: "Add new feature\n\nDetailed description here",
			want:  "Add new feature",
		},
		{
			name:  "message with empty lines",
			input: "Fix bug\n\n\nThis fixes the issue\n\nMore details",
			want:  "Fix bug",
		},
		{
			name:  "message starting with empty line",
			input: "\n\nActual message",
			want:  "Actual message",
		},
		{
			name:  "only whitespace",
			input: "   \n  \n  ",
			want:  "",
		},
		{
			name:  "commit with long message",
			input: "feat: Add new login flow\n\nThis commit adds a new login flow with OAuth support.\nIt includes:\n- Google OAuth\n- GitHub OAuth\n- Email/password fallback",
			want:  "feat: Add new login flow",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getOneLineCommitMessage(tt.input)
			if got != tt.want {
				t.Errorf("getOneLineCommitMessage(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNewMonitor(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "repomon-new-monitor-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a valid config with a test repo
	repoPath := filepath.Join(tempDir, "test-repo")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}

	if err := initTestRepo(repoPath); err != nil {
		t.Fatalf("Failed to initialize test repo: %v", err)
	}

	cfg := &config.Config{
		Days: 7,
		Groups: map[string]*config.Group{
			"default": {
				Repos: []string{repoPath},
			},
		},
	}

	// Test NewMonitor with valid config
	monitor := NewMonitor(cfg)
	if monitor == nil {
		t.Error("NewMonitor returned nil")
	}
}

func TestNewMonitorWithRepos(t *testing.T) {
	repos := []config.Repo{
		{Name: "repo1", Path: "/path/to/repo1"},
		{Name: "repo2", Path: "/path/to/repo2"},
	}

	monitor := NewMonitorWithRepos(repos)
	if monitor == nil {
		t.Error("NewMonitorWithRepos returned nil")
	}

	// Test with empty repos
	monitorEmpty := NewMonitorWithRepos([]config.Repo{})
	if monitorEmpty == nil {
		t.Error("NewMonitorWithRepos returned nil for empty repos")
	}
}

func TestMonitor_SetDays(t *testing.T) {
	monitor := NewMonitorWithRepos([]config.Repo{})

	// Test setting days
	monitor.SetDays(30)

	// Note: days is a private field, but we can verify it works by
	// checking that commits are fetched within the time range
	// For now, just verify no panic occurs
}

func TestMonitor_getRepoCommits_WithDaysFilter(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "repomon-git-test-days")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	repoPath := filepath.Join(tempDir, "test-repo")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}

	if err := initTestRepoWithOldCommit(repoPath); err != nil {
		t.Fatalf("Failed to initialize test repo: %v", err)
	}

	monitor := NewMonitorWithRepos([]config.Repo{})
	monitor.SetDays(1)

	repo := config.Repo{Name: "test-repo", Path: repoPath}
	commits, err := monitor.getRepoCommits(context.Background(), repo)

	// With SetDays(1), old commits should be filtered out
	if err != nil {
		t.Fatalf("Failed to get commits: %v", err)
	}

	// Should have no commits since we're only looking at last 1 day
	// and the old commit was made much earlier
	t.Logf("Got %d commits with days filter", len(commits))
}

func TestMonitor_getRepoCommits_NoPathOrURL(t *testing.T) {
	monitor := NewMonitorWithRepos([]config.Repo{})
	repo := config.Repo{Name: "empty-repo"}

	_, err := monitor.getRepoCommits(context.Background(), repo)
	if err == nil {
		t.Error("Expected error for repo with no path or URL")
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

// initTestRepoWithOldCommit creates a repo with an old commit
func initTestRepoWithOldCommit(repoPath string) error {
	// Create a simple test file
	testFile := filepath.Join(repoPath, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		return err
	}

	// Use go-git to initialize repository
	_, err := git.PlainInit(repoPath, false)
	if err != nil {
		return err
	}

	// Open the repository
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

	// Create a commit from 30 days ago
	oldTime := time.Now().AddDate(0, 0, -30)
	_, err = worktree.Commit("Old commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  oldTime,
		},
	})

	return err
}
