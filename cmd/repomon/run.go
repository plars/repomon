package main

import (
	"context"
	"fmt"
	"log/slog"
)

// runOptions holds the flags specific to the run command.
type runOptions struct {
	days  int
	debug bool
}

// executeRun contains the core logic for the default run command.
func (r *repomonRunner) executeRun(ctx context.Context, args []string, runOpts *runOptions, rootOpts *rootOptions) error {
	// Set up a logger that writes to errorWriter for this function's scope.
	logger := slog.New(slog.NewTextHandler(r.err, nil))

	cfg, err := r.loadConfig(rootOpts.configFile)
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Only override cfg.Days if runOpts.days was explicitly changed from its default (1)
	// and is different from the config's default.
	if runOpts.days != 1 || cfg.Days == 0 {
		if runOpts.days != 1 {
			cfg.Days = runOpts.days
		} else if cfg.Days == 0 {
			cfg.Days = 1
		}
	}

	if runOpts.debug {
		logger = slog.New(slog.NewTextHandler(r.err, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}

	requestedGroupName := rootOpts.group
	if requestedGroupName == "" {
		requestedGroupName = "default"
	}

	repos, _, err := cfg.GetRepos(requestedGroupName)
	if err != nil {
		logger.Error("Failed to get repositories", "error", err)
		return fmt.Errorf("failed to get repositories: %w", err)
	}

	monitor := r.newGitMonitor(repos)
	monitor.SetDays(cfg.Days)
	results, err := monitor.GetRecentCommits(ctx)
	if err != nil {
		logger.Error("Failed to get recent commits", "error", err)
		return fmt.Errorf("failed to get recent commits: %w", err)
	}

	reporter := r.newFormatter()
	output, err := reporter.Format(results)
	if err != nil {
		logger.Error("Failed to format report", "error", err)
		return fmt.Errorf("failed to format report: %w", err)
	}

	fmt.Fprint(r.output, output)
	return nil
}
