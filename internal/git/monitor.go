package git

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"log/slog"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/plars/repomon/internal/config"
	"github.com/schollz/progressbar/v3"
)

const maxConcurrentRepos = 10

// Commit represents a git commit
type Commit struct {
	Hash      string
	Message   string
	Author    string
	Timestamp time.Time
}

// RepoResult represents result for a single repository
type RepoResult struct {
	Repo    config.Repo
	Commits []Commit
	Error   error
}

// GitCloner defines the interface for cloning git repositories.
// Clone returns the path to the cloned repo and a cleanup function to call when done.
type GitCloner interface {
	Clone(ctx context.Context, repoURL, branch string) (repoPath string, cleanup func(), err error)
}

// RealGitCloner implements GitCloner using the git binary
type RealGitCloner struct{}

func (c *RealGitCloner) Clone(ctx context.Context, repoURL, branch string) (string, func(), error) {
	tempDir, err := os.MkdirTemp("", "repomon-*")
	if err != nil {
		return "", func() {}, fmt.Errorf("failed to create temp directory: %w", err)
	}
	cleanup := func() { os.RemoveAll(tempDir) }

	args := []string{
		"-c", "filter.lfs.smudge=",
		"-c", "filter.lfs.clean=",
		"-c", "filter.lfs.process=",
		"-c", "filter.lfs.required=false",
		"clone", repoURL, tempDir, "--depth", "100", "--no-tags",
	}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("git clone failed: %w: %s", err, output)
	}
	return tempDir, cleanup, nil
}

// CachingGitCloner implements GitCloner with local caching
type CachingGitCloner struct {
	cacheDir string
}

// NewCachingGitCloner creates a CachingGitCloner
func NewCachingGitCloner(cacheDir string) *CachingGitCloner {
	return &CachingGitCloner{
		cacheDir: cacheDir,
	}
}

func (c *CachingGitCloner) Clone(ctx context.Context, repoURL, branch string) (string, func(), error) {
	cacheName := sanitizeRepoName(repoURL, branch)
	cachePath := filepath.Join(c.cacheDir, cacheName)

	if _, err := os.Stat(cachePath); err == nil {
		slog.Debug("Using cached repository", "path", cachePath)
		if err := c.fetchUpdates(ctx, cachePath, branch); err != nil {
			slog.Warn("Fetch failed, re-cloning", "error", err)
			if err := os.RemoveAll(cachePath); err != nil {
				slog.Warn("Failed to remove broken cache", "error", err)
			}
		} else {
			return cachePath, func() {}, nil
		}
	}

	if err := c.cloneToCache(ctx, repoURL, cachePath, branch); err != nil {
		return "", func() {}, err
	}

	return cachePath, func() {}, nil
}

func (c *CachingGitCloner) fetchUpdates(ctx context.Context, repoPath, branch string) error {
	cmd := exec.CommandContext(ctx, "git", "fetch", "origin")
	cmd.Dir = repoPath
	cmd.Env = append(os.Environ(), "GIT_LFS_SKIP_SMUDGE=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git fetch failed: %w: %s", err, output)
	}

	// Reset working tree to match the fetched remote state
	resetRef := "origin/HEAD"
	if branch != "" {
		resetRef = "origin/" + branch
	}
	cmd = exec.CommandContext(ctx, "git", "reset", "--hard", resetRef)
	cmd.Dir = repoPath
	cmd.Env = append(os.Environ(), "GIT_LFS_SKIP_SMUDGE=1")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git reset failed: %w: %s", err, output)
	}
	return nil
}

func (c *CachingGitCloner) cloneToCache(ctx context.Context, repoURL, cachePath, branch string) error {
	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	args := []string{
		"-c", "filter.lfs.smudge=",
		"-c", "filter.lfs.clean=",
		"-c", "filter.lfs.process=",
		"-c", "filter.lfs.required=false",
		"clone", repoURL, cachePath, "--depth", "100", "--no-tags",
	}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone failed: %w: %s", err, output)
	}
	return nil
}

func sanitizeRepoName(repoURL, branch string) string {
	key := repoURL
	if branch != "" {
		key = key + "#" + branch
	}
	h := sha256.Sum256([]byte(key))
	// Use repo basename for readability + hash prefix for uniqueness
	base := strings.TrimSuffix(filepath.Base(repoURL), ".git")
	return fmt.Sprintf("%s-%x", base, h[:8])
}

type Monitor struct {
	repos  []config.Repo
	days   int
	cloner GitCloner
}

func NewMonitor(cfg *config.Config) *Monitor {
	repos, _, err := cfg.GetRepos("default") // Handle the new error return
	if err != nil {
		slog.Error("Failed to get default repos for monitor initialization", "error", err)
		return &Monitor{repos: []config.Repo{}, days: 1} // Return empty monitor on error
	}
	return NewMonitorWithRepos(repos)
}

func NewMonitorWithRepos(repos []config.Repo) *Monitor {
	return &Monitor{
		repos:  repos,
		days:   1,
		cloner: &RealGitCloner{},
	}
}

// NewMonitorWithCache creates a Monitor with optional caching based on config
func NewMonitorWithCache(repos []config.Repo, cacheEnabled bool, cacheDir string) *Monitor {
	var cloner GitCloner = &RealGitCloner{}

	if cacheEnabled {
		if cacheDir == "" {
			// Default cache directory
			home, err := os.UserHomeDir()
			if err != nil {
				slog.Warn("Failed to get home directory for cache", "error", err)
			} else {
				cacheDir = filepath.Join(home, ".cache", "repomon")
			}
		}
		if cacheDir != "" {
			cloner = NewCachingGitCloner(cacheDir)
		}
	}

	return &Monitor{
		repos:  repos,
		days:   1,
		cloner: cloner,
	}
}

// NewMonitorWithCloner creates a Monitor with a custom GitCloner for testing
func NewMonitorWithCloner(repos []config.Repo, cloner GitCloner) *Monitor {
	return &Monitor{
		repos:  repos,
		days:   1,
		cloner: cloner,
	}
}

func (m *Monitor) SetDays(days int) {
	m.days = days
}

func (m *Monitor) GetRecentCommits(ctx context.Context) ([]RepoResult, error) {
	results := make([]RepoResult, len(m.repos))
	var wg sync.WaitGroup

	// Use a semaphore to limit concurrent goroutines
	sem := make(chan struct{}, maxConcurrentRepos)

	bar := progressbar.NewOptions(len(m.repos),
		progressbar.OptionSetDescription("Fetching commits"),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWriter(os.Stderr),
	)

	for i, repo := range m.repos {
		wg.Add(1)
		go func(index int, repo config.Repo) {
			defer wg.Done()
			defer func() { _ = bar.Add(1) }()

			sem <- struct{}{}        // Acquire
			defer func() { <-sem }() // Release

			result := RepoResult{Repo: repo}
			commits, err := m.getRepoCommits(ctx, repo)
			if err != nil {
				location := repo.Path
				if repo.URL != "" {
					location = repo.URL
				}
				slog.Debug("Failed to get commits for repository",
					"repo", repo.Name,
					"location", location,
					"error", err)
				result.Error = err
			} else {
				result.Commits = commits
				slog.Debug("Retrieved commits for repository",
					"repo", repo.Name,
					"commits", len(commits))
			}
			results[index] = result
		}(i, repo)
	}

	wg.Wait()
	_ = bar.Finish()
	return results, nil
}

// getRepoCommits retrieves recent commits for a single repository
func (m *Monitor) getRepoCommits(ctx context.Context, repo config.Repo) ([]Commit, error) {
	var gitRepo *git.Repository
	var err error

	// Determine if this is a remote or local repository
	if repo.URL != "" {
		var cleanup func()
		gitRepo, cleanup, err = m.cloneRemoteRepo(ctx, repo.URL, repo.Branch)
		if err != nil {
			return nil, fmt.Errorf("failed to clone remote repository: %w", err)
		}
		defer cleanup()
	} else if repo.Path != "" {
		// Local repository - check if path exists
		if _, err := os.Stat(repo.Path); os.IsNotExist(err) {
			return nil, fmt.Errorf("repository path does not exist: %s", repo.Path)
		}

		// Open local git repository
		gitRepo, err = git.PlainOpen(repo.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to open git repository: %w", err)
		}
	} else {
		// Neither URL nor Path provided
		return nil, fmt.Errorf("repository configuration must specify either 'path' or 'url'")
	}

	// Get reference to branch or HEAD
	var ref *plumbing.Reference
	if repo.Branch != "" {
		// Try to resolve branch
		ref, err = gitRepo.Reference(plumbing.NewBranchReferenceName(repo.Branch), true)
		if err != nil {
			// Fallback to resolving the name directly if it's not a simple branch name
			ref, err = gitRepo.Reference(plumbing.ReferenceName(repo.Branch), true)
			if err != nil {
				slog.Debug("Failed to resolve branch reference", "branch", repo.Branch, "error", err)
				return nil, fmt.Errorf("failed to resolve branch '%s': %w", repo.Branch, err)
			}
		}
	} else {
		ref, err = gitRepo.Head()
		if err != nil {
			slog.Debug("Failed to get HEAD reference", "error", err)
			return nil, fmt.Errorf("failed to get HEAD reference: %w", err)
		}
	}
	slog.Debug("Got reference for commit retrieval", "hash", ref.Hash(), "name", ref.Name())

	// Get commit history
	commitIter, err := gitRepo.Log(&git.LogOptions{
		From:  ref.Hash(),
		Order: git.LogOrderCommitterTime,
	})
	if err != nil {
		slog.Debug("Failed to get commit history", "error", err, "ref", ref.Hash())
		return nil, fmt.Errorf("failed to get commit history: %w", err)
	}
	defer commitIter.Close()

	cutoff := time.Now().AddDate(0, 0, -m.days)

	var commits []Commit
	err = commitIter.ForEach(func(c *object.Commit) error {
		if c.Author.When.Before(cutoff) {
			return storer.ErrStop
		}

		message := getOneLineCommitMessage(c.Message)
		commits = append(commits, Commit{
			Hash:      c.Hash.String(),
			Message:   message,
			Author:    c.Author.Name,
			Timestamp: c.Author.When,
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to iterate commits: %w", err)
	}

	return commits, nil
}

// getOneLineCommitMessage extracts the first line of a commit message (like git log --oneline)
func getOneLineCommitMessage(message string) string {
	// Split by newlines and take the first non-empty line
	lines := strings.Split(message, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	// Fallback to full message if no lines found
	return strings.TrimSpace(message)
}

// cloneRemoteRepo obtains a git repository for a remote URL using the configured GitCloner.
func (m *Monitor) cloneRemoteRepo(ctx context.Context, repoURL, branch string) (*git.Repository, func(), error) {
	repoPath, cleanup, err := m.cloner.Clone(ctx, repoURL, branch)
	if err != nil {
		slog.Debug("Failed to clone remote repository", "error", err, "url", repoURL, "branch", branch)
		return nil, func() {}, fmt.Errorf("git clone failed: %w", err)
	}

	gitRepo, err := git.PlainOpen(repoPath)
	if err != nil {
		cleanup()
		return nil, func() {}, fmt.Errorf("failed to open cloned repository: %w", err)
	}

	slog.Debug("Successfully opened remote repository", "url", repoURL, "path", repoPath)
	return gitRepo, cleanup, nil
}
