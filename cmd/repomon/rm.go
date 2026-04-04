package main

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// rmOptions holds the flags specific to the 'rm' command.
type rmOptions struct {
	force bool
}

func (r *repomonRunner) rmCmd(rootOpts *rootOptions) *cobra.Command {
	rmOpts := &rmOptions{}

	cmd := &cobra.Command{
		Use:   "rm <repo>",
		Short: "Removes a repository from the configuration",
		Long: `Removes a repository from the configuration. The repository can be
identified by its short name (as shown in 'list') or by its full path/URL.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := r.executeRm(args, rootOpts, rmOpts); err != nil {
				slog.Error("Remove command failed", "error", err)
				os.Exit(1)
			}
		},
	}
	cmd.Flags().BoolVarP(&rmOpts.force, "force", "f", false, "skip confirmation prompt")
	return cmd
}

// executeRm contains the core logic for the 'rm' command.
func (r *repomonRunner) executeRm(args []string, rootOpts *rootOptions, rmOpts *rmOptions) error {
	repoIdentifier := args[0]
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

	// If not forced, prompt for confirmation
	if !rmOpts.force {
		fmt.Fprintf(r.output, "Remove '%s' from group '%s'? [y/N]: ", repoIdentifier, requestedGroupName)
		reader := bufio.NewReader(r.stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Fprintf(r.output, "Cancelled.\n")
			return nil
		}
	}

	removed, err := cfg.RemoveRepo(repoIdentifier, requestedGroupName)
	if err != nil {
		logger.Error("Failed to remove repository", "error", err)
		return fmt.Errorf("failed to remove repository: %w", err)
	}

	configPath, err := resolveConfigPath(rootOpts.configFile)
	if err != nil {
		return err
	}

	if err := cfg.Save(configPath); err != nil {
		logger.Error("Failed to save configuration", "error", err)
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Fprintf(r.output, "Removed '%s' from group '%s' in %s\n", removed, requestedGroupName, configPath)
	return nil
}
