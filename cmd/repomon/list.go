package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

func (r *repomonRunner) listCmd(rootOpts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Lists configured git repositories",
		Run: func(cmd *cobra.Command, args []string) {
			if err := r.executeList(args, rootOpts); err != nil {
				slog.Error("List command failed", "error", err)
				os.Exit(1)
			}
		},
	}
}

// executeList contains the core logic for the 'list' command.
func (r *repomonRunner) executeList(args []string, rootOpts *rootOptions) error {
	logger := slog.New(slog.NewTextHandler(r.err, nil))

	cfg, err := r.loadConfig(rootOpts.configFile)
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	requestedGroupName := rootOpts.group
	if requestedGroupName == "" {
		requestedGroupName = "default"
	}

	repos, effectiveGroupName, err := cfg.GetRepos(requestedGroupName)
	if err != nil {
		logger.Error("Failed to get repositories", "error", err)
		return fmt.Errorf("failed to get repositories: %w", err)
	}

	if len(repos) == 0 {
		fmt.Fprintf(r.output, "No repositories found for group '%s'.\n", effectiveGroupName)
		return nil
	}

	fmt.Fprintf(r.output, "Repositories for group '%s':\n", effectiveGroupName)
	for _, repo := range repos {
		repoDisplay := repo.Name
		if repo.Branch != "" {
			repoDisplay = fmt.Sprintf("%s#%s", repo.Name, repo.Branch)
		}

		if repo.Path != "" {
			fmt.Fprintf(r.output, "  - %s: %s\n", repoDisplay, repo.Path)
		} else if repo.URL != "" {
			fmt.Fprintf(r.output, "  - %s: %s (remote)\n", repoDisplay, repo.URL)
		} else {
			fmt.Fprintf(r.output, "  - %s: (unknown location)\n", repoDisplay)
		}
	}
	return nil
}
