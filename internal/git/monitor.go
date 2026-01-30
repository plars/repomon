package git

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/plars/repomon/internal/config"
	"log/slog"
)

// Commit represents a git commit
type Commit struct {
	Hash      string
	Message   string
	Timestamp time.Time
}

// RepoResult represents result for a single repository
type RepoResult struct {
	Repo    config.Repo
	Commits []Commit
	Error   error
}

// Monitor handles git repository monitoring
type Monitor struct {
	config *config.Config
}

// NewMonitor creates a new git monitor
func NewMonitor(cfg *config.Config) *Monitor {
	return &Monitor{
		config: cfg,
	}
}

// GetRecentCommits retrieves recent commits from all configured repositories
func (m *Monitor) GetRecentCommits(ctx context.Context) ([]RepoResult, error) {
	results := make([]RepoResult, len(m.config.Repos))
	var wg sync.WaitGroup

	// Use a semaphore to limit concurrent goroutines
	sem := make(chan struct{}, 10) // Limit to 10 concurrent operations

	for i, repo := range m.config.Repos {
		wg.Add(1)
		go func(index int, repo config.Repo) {
			defer wg.Done()
			sem <- struct{}{}        // Acquire
			defer func() { <-sem }() // Release

			result := RepoResult{Repo: repo}
			commits, err := m.getRepoCommits(ctx, repo)
			if err != nil {
				location := repo.Path
				if repo.URL != "" {
					location = repo.URL
				}
				slog.Warn("Failed to get commits for repository",
					"repo", repo.Name,
					"location", location,
					"error", err)
				result.Error = err
			} else {
				result.Commits = commits
				slog.Info("Retrieved commits for repository",
					"repo", repo.Name,
					"commits", len(commits))
			}
			results[index] = result
		}(i, repo)
	}

	wg.Wait()
	return results, nil
}

// getRepoCommits retrieves recent commits for a single repository
func (m *Monitor) getRepoCommits(ctx context.Context, repo config.Repo) ([]Commit, error) {
	var gitRepo *git.Repository
	var err error

	// Determine if this is a remote or local repository
	if repo.URL != "" {
		// Remote repository - use shallow clone with memory storage
		gitRepo, err = m.cloneRemoteRepo(ctx, repo.URL)
		if err != nil {
			return nil, fmt.Errorf("failed to clone remote repository: %w", err)
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

	// Get reference to HEAD
	ref, err := gitRepo.Head()
	if err != nil {
		slog.Debug("Failed to get HEAD reference", "error", err)
		return nil, fmt.Errorf("failed to get HEAD reference: %w", err)
	}
	slog.Debug("Got HEAD reference", "hash", ref.Hash(), "name", ref.Name())

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

	// Calculate the cutoff date
	cutoff := time.Now().AddDate(0, 0, -m.config.Defaults.Days)

	var commits []Commit
	err = commitIter.ForEach(func(c *object.Commit) error {
		// Check if we've reached the cutoff date
		if c.Author.When.Before(cutoff) {
			return fmt.Errorf("stop iteration") // Stop iteration
		}

		// Only include one-line commit message
		message := getOneLineCommitMessage(c.Message)
		commits = append(commits, Commit{
			Hash:      c.Hash.String(),
			Message:   message,
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

// cloneRemoteRepo performs a shallow clone of a remote repository
func (m *Monitor) cloneRemoteRepo(ctx context.Context, url string) (*git.Repository, error) {
	// Use memory storage to avoid writing to disk
	storage := memory.NewStorage()

	// Perform clone with reasonable depth limit for efficiency
	repo, err := git.CloneContext(ctx, storage, nil, &git.CloneOptions{
		URL:   url,
		Depth: 100,        // Reasonable depth limit while still being efficient
		Tags:  git.NoTags, // Don't fetch tags to save bandwidth
	})
	if err != nil {
		slog.Debug("Failed to clone remote repository", "error", err, "url", url)
		return nil, fmt.Errorf("failed to clone remote repository: %w", err)
	}

	slog.Debug("Successfully cloned remote repository", "url", url)
	return repo, nil
}

// parseGitLog is no longer needed with go-git, but kept for backwards compatibility
func (m *Monitor) parseGitLog(output []byte) ([]Commit, error) {
	return []Commit{}, fmt.Errorf("parseGitLog deprecated with go-git implementation")
}
