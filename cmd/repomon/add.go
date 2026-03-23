package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

func (r *repomonRunner) addCmd(rootOpts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "add <repo>",
		Short: "Adds a repository to the configuration",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := r.executeAdd(args, rootOpts); err != nil {
				slog.Error("Add command failed", "error", err)
				os.Exit(1)
			}
		},
	}
}

// executeAdd contains the core logic for the 'add' command.
func (r *repomonRunner) executeAdd(args []string, rootOpts *rootOptions) error {
	if len(args) == 0 {
		return fmt.Errorf("repository argument is required")
	}

	repoStr := args[0]
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

	if err := cfg.AddRepo(repoStr, requestedGroupName); err != nil {
		logger.Error("Failed to add repository", "error", err)
		return fmt.Errorf("failed to add repository: %w", err)
	}

	configPath, err := resolveConfigPath(rootOpts.configFile)
	if err != nil {
		return err
	}

	if err := cfg.Save(configPath); err != nil {
		logger.Error("Failed to save configuration", "error", err)
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Fprintf(r.output, "Added '%s' to group '%s' in %s\n", repoStr, requestedGroupName, configPath)
	return nil
}
