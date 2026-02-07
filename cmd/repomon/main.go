package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/plars/repomon/internal/config"
	"github.com/plars/repomon/internal/git"
	"github.com/plars/repomon/internal/report"
	"github.com/spf13/cobra"
)

// rootOptions holds the persistent flags common to all commands.
type rootOptions struct {
	configFile string
	group      string
}

// runOptions holds the flags specific to the 'run' command.
type runOptions struct {
	days  int
	debug bool
}

func main() {
	// Initialize option structs
	rootOpts := &rootOptions{}
	runOpts := &runOptions{}

	var runCmd = &cobra.Command{
		Use:   "run",
		Short: "Monitors configured git repositories and reports recent changes",
		Run: func(cmd *cobra.Command, args []string) {
			// Call the extracted handler function
			if err := executeRun(cmd.Context(), args, runOpts, rootOpts, os.Stdout, os.Stderr); err != nil {
				slog.Error("Run command failed", "error", err) // Log error before exiting
				os.Exit(1)
			}
		},
	}
	// Bind run-specific flags to runOptions
	runCmd.Flags().IntVarP(&runOpts.days, "days", "d", 1, "number of days to look back in history")
	runCmd.Flags().BoolVar(&runOpts.debug, "debug", false, "enable debug logging")

	var rootCmd = &cobra.Command{
		Use:   "repomon",
		Short: "A tool to monitor git repositories and report recent changes",
		Long: `Repomon monitors configured git repositories and generates a report
showing the most recent commits to each repository in an easy-to-read format.`,
		Run: runCmd.Run, // Set runCmd.Run as the default action for rootCmd
	}

	// Bind persistent flags to rootOptions
	rootCmd.PersistentFlags().StringVarP(&rootOpts.configFile, "config", "c", "", "path to config file (default ~/.config/repomon/config.toml)")
	rootCmd.PersistentFlags().StringVarP(&rootOpts.group, "group", "g", "", "repository group to use (default: 'default')")
	// Add run-specific flags to rootCmd so they work without 'run' subcommand
	rootCmd.Flags().AddFlagSet(runCmd.Flags())


	var listCmd = &cobra.Command{
		Use:   "list",
		Short: "Lists configured git repositories",
		Run: func(cmd *cobra.Command, args []string) {
			// Call the extracted handler function without listOpts
			if err := executeList(args, rootOpts, os.Stdout, os.Stderr); err != nil {
				slog.Error("List command failed", "error", err) // Log error before exiting
				os.Exit(1)
			}
		},
	}

	rootCmd.AddCommand(runCmd) // Keep runCmd as a separate subcommand
	rootCmd.AddCommand(listCmd)

	if err := rootCmd.Execute(); err != nil {
		slog.Error("Command execution failed", "error", err)
		os.Exit(1)
	}
}

// executeRun contains the core logic for the 'run' command.
// It now takes outputWriter and errorWriter, and returns an error instead of os.Exit.
func executeRun(ctx context.Context, args []string, runOpts *runOptions, rootOpts *rootOptions, outputWriter io.Writer, errorWriter io.Writer) error {
	// Set up a logger that writes to errorWriter for this function's scope.
	logger := slog.New(slog.NewTextHandler(errorWriter, nil))

	cfg, err := config.Load(rootOpts.configFile)
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Only override cfg.Defaults.Days if runOpts.days was explicitly changed from its default (1)
	// and is different from the config's default.
	if runOpts.days != 1 || cfg.Defaults.Days == 0 { // if runOpts.days is not default OR config default is 0
		if runOpts.days != 1 { // if runOpts.days was explicitly set
			cfg.Defaults.Days = runOpts.days
		} else if cfg.Defaults.Days == 0 { // if runOpts.days was not set and config default is 0
			cfg.Defaults.Days = 1 // Fallback to 1 day
		}
	}


	if runOpts.debug {
		logger = slog.New(slog.NewTextHandler(errorWriter, &slog.HandlerOptions{Level: slog.LevelDebug}))
		// Note: slog.SetLogLoggerLevel(slog.LevelDebug) affects the default logger globally.
		// For tests, it's better to rely on this local 'logger' instance.
	}


	requestedGroupName := rootOpts.group
	if requestedGroupName == "" {
		requestedGroupName = "default"
	}

	repos, _, err := cfg.GetRepos(requestedGroupName) // Handle the new error return, ignore effectiveGroupName
	if err != nil {
		logger.Error("Failed to get repositories", "error", err)
		return fmt.Errorf("failed to get repositories: %w", err)
	}

	monitor := git.NewMonitorWithRepos(repos)
	monitor.SetDays(cfg.Defaults.Days)
	// Pass the context from cmd to GetRecentCommits
	results, err := monitor.GetRecentCommits(ctx)
	if err != nil {
		logger.Error("Failed to get recent commits", "error", err)
		return fmt.Errorf("failed to get recent commits: %w", err)
	}

	reporter := report.NewFormatter()
	output, err := reporter.Format(results)
	if err != nil {
		logger.Error("Failed to format report", "error", err)
		return fmt.Errorf("failed to format report: %w", err)
	}

	fmt.Fprint(outputWriter, output)
	return nil
}

// executeList contains the core logic for the 'list' command.
// It now takes outputWriter and errorWriter, and returns an error instead of os.Exit.
func executeList(args []string, rootOpts *rootOptions, outputWriter io.Writer, errorWriter io.Writer) error {
	logger := slog.New(slog.NewTextHandler(errorWriter, nil))

	cfg, err := config.Load(rootOpts.configFile)
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	requestedGroupName := rootOpts.group
	if requestedGroupName == "" {
		requestedGroupName = "default"
	}

	repos, effectiveGroupName, err := cfg.GetRepos(requestedGroupName) // Handle the new error return
	if err != nil {
		logger.Error("Failed to get repositories", "error", err)
		return fmt.Errorf("failed to get repositories: %w", err)
	}

	if len(repos) == 0 {
		fmt.Fprintf(outputWriter, "No repositories found for group '%s'.\n", effectiveGroupName)
		return nil
	}

	fmt.Fprintf(outputWriter, "Repositories for group '%s':\n", effectiveGroupName)
	for _, repo := range repos {
		if repo.Path != "" {
			fmt.Fprintf(outputWriter, "  - %s: %s\n", repo.Name, repo.Path)
		} else if repo.URL != "" {
			fmt.Fprintf(outputWriter, "  - %s: %s (remote)\n", repo.Name, repo.URL)
		} else {
			fmt.Fprintf(outputWriter, "  - %s: (unknown location)\n", repo.Name)
		}
	}
	return nil
}