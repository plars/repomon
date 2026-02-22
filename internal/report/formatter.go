package report

import (
	"fmt"
	"strings"
	"time"

	"github.com/plars/repomon/internal/git"
)

// Formatter formats repository results into human-readable reports
type Formatter struct{}

// NewFormatter creates a new report formatter
func NewFormatter() *Formatter {
	return &Formatter{}
}

// Format formats the repository results into a human-readable report
func (f *Formatter) Format(results []git.RepoResult) (string, error) {
	var sb strings.Builder

	// Header
	sb.WriteString("Repository Monitor Report\n")
	sb.WriteString("========================\n\n")

	hasAnyCommits := false

	// Process each repository in order
	for _, result := range results {
		repoHeader := fmt.Sprintf("📁 %s", result.Repo.Name)
		if result.Repo.Branch != "" {
			repoHeader = fmt.Sprintf("📁 %s (%s)", result.Repo.Name, result.Repo.Branch)
		}

		if result.Error != nil {
			sb.WriteString(repoHeader + "\n")
			sb.WriteString(fmt.Sprintf("   ❌ Error: %s\n\n", result.Error.Error()))
			continue
		}

		if len(result.Commits) == 0 {
			sb.WriteString(repoHeader + "\n")
			sb.WriteString("   ✅ No recent commits\n\n")
			continue
		}

		hasAnyCommits = true
		sb.WriteString(repoHeader + "\n")
		sb.WriteString("   Recent commits:\n")

		for _, commit := range result.Commits {
			timeStr := f.formatRelativeTime(commit.Timestamp)
			sb.WriteString(fmt.Sprintf("   • %s - %s (%s)\n", commit.Message, commit.Author, timeStr))
		}
		sb.WriteString("\n")
	}

	if !hasAnyCommits {
		sb.WriteString("No recent commits found in any repository.\n")
	}

	return sb.String(), nil
}

// formatRelativeTime formats a timestamp as relative time
func (f *Formatter) formatRelativeTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	if diff < time.Hour {
		minutes := int(diff.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	}

	if diff < 24*time.Hour {
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	}

	if diff < 7*24*time.Hour {
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}

	// For older commits, just show the date
	return t.Format("2006-01-02")
}
