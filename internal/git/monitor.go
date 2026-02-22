package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"log/slog"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/plars/repomon/internal/config"
	"github.com/schollz/progressbar/v3"
)

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

// GitCloner defines the interface for cloning git repositories
type GitCloner interface {
	Clone(ctx context.Context, repoURL, targetDir string, branch string) error
}

// RealGitCloner implements GitCloner using the git binary
type RealGitCloner struct{}

func (c *RealGitCloner) Clone(ctx context.Context, repoURL, targetDir string, branch string) error {
	args := []string{"clone", repoURL, targetDir, "--depth", "100", "--no-tags"}
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
	sem := make(chan struct{}, 10) // Limit to 10 concurrent operations

	bar := progressbar.NewOptions(len(m.repos),
		progressbar.OptionSetDescription("Fetching commits"),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWriter(os.Stderr),
	)

	for i, repo := range m.repos {
		wg.Add(1)
		go func(index int, repo config.Repo) {
			defer wg.Done()
			defer bar.Add(1)

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
	bar.Finish()
	return results, nil
}

// getRepoCommits retrieves recent commits for a single repository
func (m *Monitor) getRepoCommits(ctx context.Context, repo config.Repo) ([]Commit, error) {
	var gitRepo *git.Repository
	var err error
	var tempDir string

	// Determine if this is a remote or local repository
	if repo.URL != "" {
		// Remote repository - use git binary for cloning (supports credential helpers)
		gitRepo, tempDir, err = m.cloneRemoteRepo(ctx, repo.URL, repo.Branch)
		if err != nil {
			return nil, fmt.Errorf("failed to clone remote repository: %w", err)
		}
		// Clean up temp directory when done reading commits
		if tempDir != "" {
			defer os.RemoveAll(tempDir)
		}
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
		// Check if we've reached the cutoff date
		if c.Author.When.Before(cutoff) {
			return fmt.Errorf("stop iteration")
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

	// Handle iteration completion vs error
	if err != nil && err.Error() != "stop iteration" {
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

// cloneRemoteRepo performs a shallow clone using the configured GitCloner
func (m *Monitor) cloneRemoteRepo(ctx context.Context, repoURL string, branch string) (*git.Repository, string, error) {
	tempDir, err := os.MkdirTemp("", "repomon-*")
	if err != nil {
		return nil, "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	err = m.cloner.Clone(ctx, repoURL, tempDir, branch)
	if err != nil {
		os.RemoveAll(tempDir)
		slog.Debug("Failed to clone remote repository", "error", err, "url", repoURL, "branch", branch)
		return nil, "", fmt.Errorf("git clone failed: %w", err)
	}

	gitRepo, err := git.PlainOpen(tempDir)
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, "", fmt.Errorf("failed to open cloned repository: %w", err)
	}

	slog.Debug("Successfully cloned remote repository", "url", repoURL, "tempDir", tempDir)
	return gitRepo, tempDir, nil
}

// parseGitLog is no longer needed with go-git, but kept for backwards compatibility
func (m *Monitor) parseGitLog(output []byte) ([]Commit, error) {
	return []Commit{}, fmt.Errorf("parseGitLog deprecated with go-git implementation")
}
