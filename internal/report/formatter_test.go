package report

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/plars/repomon/internal/config"
	"github.com/plars/repomon/internal/git"
)

func TestFormatter_Format(t *testing.T) {
	formatter := NewFormatter()

	// Test case 1: Results with commits and errors
	results := []git.RepoResult{
		{
			Repo: config.Repo{Name: "repo-with-commits", Path: "/path/to/repo1"},
			Commits: []git.Commit{
				{
					Hash:      "abc123",
					Message:   "Add new feature",
					Timestamp: time.Now().Add(-1 * time.Hour),
				},
				{
					Hash:      "def456",
					Message:   "Fix bug",
					Timestamp: time.Now().Add(-2 * time.Hour),
				},
			},
			Error: nil,
		},
		{
			Repo:    config.Repo{Name: "repo-with-error", Path: "/non/existent"},
			Commits: []git.Commit{},
			Error:   fmt.Errorf("repository not found"),
		},
		{
			Repo:    config.Repo{Name: "repo-no-commits", Path: "/path/to/repo3"},
			Commits: []git.Commit{},
			Error:   nil,
		},
	}

	output, err := formatter.Format(results)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check that all repositories are included
	if !strings.Contains(output, "repo-with-commits") {
		t.Error("Output should contain 'repo-with-commits'")
	}
	if !strings.Contains(output, "repo-with-error") {
		t.Error("Output should contain 'repo-with-error'")
	}
	if !strings.Contains(output, "repo-no-commits") {
		t.Error("Output should contain 'repo-no-commits'")
	}

	// Check commit messages are included
	if !strings.Contains(output, "Add new feature") {
		t.Error("Output should contain 'Add new feature'")
	}
	if !strings.Contains(output, "Fix bug") {
		t.Error("Output should contain 'Fix bug'")
	}

	// Check error message is included
	if !strings.Contains(output, "repository not found") {
		t.Error("Output should contain error message")
	}

	// Check no commits message
	if !strings.Contains(output, "No recent commits") {
		t.Error("Output should contain 'No recent commits'")
	}
}

func TestFormatter_Format_NoCommits(t *testing.T) {
	formatter := NewFormatter()

	results := []git.RepoResult{
		{
			Repo:    config.Repo{Name: "empty-repo", Path: "/path/to/empty"},
			Commits: []git.Commit{},
			Error:   nil,
		},
	}

	output, err := formatter.Format(results)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !strings.Contains(output, "No recent commits found in any repository") {
		t.Error("Output should contain no commits message")
	}
}

func TestFormatter_formatRelativeTime(t *testing.T) {
	formatter := NewFormatter()

	// Test minutes ago
	minutesAgo := time.Now().Add(-30 * time.Minute)
	result := formatter.formatRelativeTime(minutesAgo)
	if result != "30 minutes ago" {
		t.Errorf("Expected '30 minutes ago', got '%s'", result)
	}

	// Test 1 minute ago (singular)
	oneMinuteAgo := time.Now().Add(-1 * time.Minute)
	result = formatter.formatRelativeTime(oneMinuteAgo)
	if result != "1 minute ago" {
		t.Errorf("Expected '1 minute ago', got '%s'", result)
	}

	// Test hours ago
	hoursAgo := time.Now().Add(-3 * time.Hour)
	result = formatter.formatRelativeTime(hoursAgo)
	if result != "3 hours ago" {
		t.Errorf("Expected '3 hours ago', got '%s'", result)
	}

	// Test 1 hour ago (singular)
	oneHourAgo := time.Now().Add(-1 * time.Hour)
	result = formatter.formatRelativeTime(oneHourAgo)
	if result != "1 hour ago" {
		t.Errorf("Expected '1 hour ago', got '%s'", result)
	}

	// Test days ago
	daysAgo := time.Now().Add(-5 * 24 * time.Hour)
	result = formatter.formatRelativeTime(daysAgo)
	if result != "5 days ago" {
		t.Errorf("Expected '5 days ago', got '%s'", result)
	}

	// Test 1 day ago (singular)
	oneDayAgo := time.Now().Add(-24 * time.Hour)
	result = formatter.formatRelativeTime(oneDayAgo)
	if result != "1 day ago" {
		t.Errorf("Expected '1 day ago', got '%s'", result)
	}

	// Test older dates (should show date format)
	olderDate := time.Now().Add(-10 * 24 * time.Hour)
	result = formatter.formatRelativeTime(olderDate)
	expected := olderDate.Format("2006-01-02")
	if result != expected {
		t.Errorf("Expected date format '%s', got '%s'", expected, result)
	}
}
