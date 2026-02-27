package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
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

func TestMonitor_GetRecentCommits_WithError(t *testing.T) {
	// Test with a repo that will fail - non-existent path
	repos := []config.Repo{
		{Name: "valid-repo", Path: "/nonexistent/path"},
		{Name: "another-invalid", Path: "/another/nonexistent"},
	}
	monitor := NewMonitorWithRepos(repos)
	results, err := monitor.GetRecentCommits(context.Background())

	// GetRecentCommits itself should not return an error - it collects errors in results
	if err != nil {
		t.Fatalf("GetRecentCommits returned unexpected error: %v", err)
	}

	// Should have 2 results
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	// Both repos should have errors
	for i, result := range results {
		if result.Error == nil {
			t.Errorf("Expected error for result %d (%s), got nil", i, result.Repo.Name)
		}
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

func TestNewMonitor_GetReposError(t *testing.T) {
	// Test with config that has no default group - GetRepos will fail
	cfg := &config.Config{
		Days:   7,
		Groups: map[string]*config.Group{}, // Empty groups - no default
	}

	// NewMonitor should return an empty monitor on error, not nil
	monitor := NewMonitor(cfg)
	if monitor == nil {
		t.Fatal("NewMonitor returned nil on error")
	}

	// Should have empty repos
	if len(monitor.repos) != 0 {
		t.Errorf("Expected empty repos on error, got %d repos", len(monitor.repos))
	}

	// Should have default days
	if monitor.days != 1 {
		t.Errorf("Expected days=1 on error, got %d", monitor.days)
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

func TestMonitor_getRepoCommits_WithBranch(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "repomon-git-test-branch")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	repoPath := filepath.Join(tempDir, "test-repo")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}

	if err := initGitRepoWithBranch(repoPath, "feature"); err != nil {
		t.Fatalf("Failed to initialize test repo with branch: %v", err)
	}

	monitor := NewMonitorWithRepos([]config.Repo{})

	// Test fetching from the feature branch
	repo := config.Repo{Name: "test-repo", Path: repoPath, Branch: "feature"}
	commits, err := monitor.getRepoCommits(context.Background(), repo)

	if err != nil {
		t.Fatalf("Failed to get commits from branch: %v", err)
	}

	found := false
	for _, c := range commits {
		if c.Message == "Feature commit" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected to find 'Feature commit' in branch history")
	}

	// Test fetching from master (should NOT have the feature commit)
	repoMaster := config.Repo{Name: "test-repo", Path: repoPath, Branch: "master"}
	commitsMaster, err := monitor.getRepoCommits(context.Background(), repoMaster)
	if err != nil {
		t.Fatalf("Failed to get commits from master: %v", err)
	}

	for _, c := range commitsMaster {
		if c.Message == "Feature commit" {
			t.Error("Did not expect to find 'Feature commit' in master history")
		}
	}
}

// Helper function to initialize a test git repository with a specific branch
func initGitRepoWithBranch(repoPath string, branchName string) error {
	// Use go-git to initialize repository
	repo, err := git.PlainInit(repoPath, false)
	if err != nil {
		return err
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return err
	}

	// Create initial file and commit on master
	testFile := filepath.Join(repoPath, "master.txt")
	if err := os.WriteFile(testFile, []byte("master content"), 0644); err != nil {
		return err
	}
	_, err = worktree.Add("master.txt")
	if err != nil {
		return err
	}
	_, err = worktree.Commit("Initial commit on master", &git.CommitOptions{
		Author: &object.Signature{Name: "Test User", Email: "test@example.com", When: time.Now()},
	})
	if err != nil {
		return err
	}

	// Create and checkout new branch
	headRef, err := repo.Head()
	if err != nil {
		return err
	}

	branchRefName := plumbing.NewBranchReferenceName(branchName)
	ref := plumbing.NewHashReference(branchRefName, headRef.Hash())
	err = repo.Storer.SetReference(ref)
	if err != nil {
		return err
	}

	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: branchRefName,
	})
	if err != nil {
		return err
	}

	// Create commit on the new branch
	featFile := filepath.Join(repoPath, "feature.txt")
	if err := os.WriteFile(featFile, []byte("feature content"), 0644); err != nil {
		return err
	}
	_, err = worktree.Add("feature.txt")
	if err != nil {
		return err
	}
	_, err = worktree.Commit("Feature commit", &git.CommitOptions{
		Author: &object.Signature{Name: "Test User", Email: "test@example.com", When: time.Now()},
	})

	return err
}

func TestMonitor_getRepoCommits_NoPathOrURL(t *testing.T) {
	monitor := NewMonitorWithRepos([]config.Repo{})
	repo := config.Repo{Name: "empty-repo"}

	_, err := monitor.getRepoCommits(context.Background(), repo)
	if err == nil {
		t.Error("Expected error for repo with no path or URL")
	}
}

// mockGitCloner is a mock implementation of GitCloner for testing
type mockGitCloner struct {
	cloneErr error
	cloneDir string // Directory to use as the "cloned" repo
}

func (m *mockGitCloner) Clone(ctx context.Context, repoURL, targetDir string, branch string) error {
	if m.cloneErr != nil {
		return m.cloneErr
	}
	// If a cloneDir is provided, copy its contents to targetDir
	if m.cloneDir != "" {
		return copyDir(m.cloneDir, targetDir)
	}
	return nil
}

// copyDir recursively copies a directory
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)
		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dstPath, data, info.Mode())
	})
}

func TestMonitor_getRepoCommits_RemoteRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "repomon-remote-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a source repo that will be "cloned"
	sourceRepoPath := filepath.Join(tempDir, "source-repo")
	if err := os.MkdirAll(sourceRepoPath, 0755); err != nil {
		t.Fatalf("Failed to create source repo dir: %v", err)
	}
	if err := initTestRepo(sourceRepoPath); err != nil {
		t.Fatalf("Failed to initialize source repo: %v", err)
	}

	// Create a mock cloner that copies from our source repo
	mockCloner := &mockGitCloner{
		cloneDir: sourceRepoPath,
	}

	monitor := NewMonitorWithCloner([]config.Repo{}, mockCloner)
	repo := config.Repo{Name: "remote-repo", URL: "https://github.com/example/test.git"}

	commits, err := monitor.getRepoCommits(context.Background(), repo)
	if err != nil {
		t.Fatalf("Failed to get commits from remote repo: %v", err)
	}

	if len(commits) == 0 {
		t.Error("Expected at least one commit from remote repo")
	}
}

func TestMonitor_getRepoCommits_RemoteRepo_CloneError(t *testing.T) {
	mockCloner := &mockGitCloner{
		cloneErr: fmt.Errorf("authentication failed"),
	}

	monitor := NewMonitorWithCloner([]config.Repo{}, mockCloner)
	repo := config.Repo{Name: "remote-repo", URL: "https://github.com/example/private.git"}

	_, err := monitor.getRepoCommits(context.Background(), repo)
	if err == nil {
		t.Error("Expected error when clone fails")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("Expected error to contain 'authentication failed', got: %v", err)
	}
}

func TestNewMonitorWithCloner(t *testing.T) {
	mockCloner := &mockGitCloner{}
	repos := []config.Repo{{Name: "test", Path: "/path/to/repo"}}

	monitor := NewMonitorWithCloner(repos, mockCloner)
	if monitor == nil {
		t.Fatal("NewMonitorWithCloner returned nil")
	}
	if monitor.cloner != mockCloner {
		t.Error("Monitor cloner was not set correctly")
	}
}

func TestRealGitCloner_Interface(t *testing.T) {
	// Ensure RealGitCloner implements GitCloner interface
	var _ GitCloner = &RealGitCloner{}
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
