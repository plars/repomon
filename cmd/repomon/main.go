package main

import (
	"fmt"
	"os"

	"github.com/plars/repomon/internal/config"
	"github.com/plars/repomon/internal/git"
	"github.com/plars/repomon/internal/report"
	"github.com/spf13/cobra"
	"log/slog"
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
			executeRun(cmd, args, runOpts, rootOpts)
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
			executeList(cmd, args, rootOpts)
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
func executeRun(cmd *cobra.Command, args []string, runOpts *runOptions, rootOpts *rootOptions) {
	cfg, err := config.Load(rootOpts.configFile)
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	if cmd.Flags().Changed("days") {
		cfg.Defaults.Days = runOpts.days
	}

	if runOpts.debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	groupName := rootOpts.group
	if groupName == "" {
		groupName = "default"
	}

	repos := cfg.GetRepos(groupName)

	monitor := git.NewMonitorWithRepos(repos)
	monitor.SetDays(cfg.Defaults.Days)
	results, err := monitor.GetRecentCommits(cmd.Context())
	if err != nil {
		slog.Error("Failed to get recent commits", "error", err)
		os.Exit(1)
	}

	reporter := report.NewFormatter()
	output, err := reporter.Format(results)
	if err != nil {
		slog.Error("Failed to format report", "error", err)
		os.Exit(1)
	}

	fmt.Print(output)
}

// executeList contains the core logic for the 'list' command.
// No longer accepts listOpts.
func executeList(cmd *cobra.Command, args []string, rootOpts *rootOptions) {
	cfg, err := config.Load(rootOpts.configFile)
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	groupName := rootOpts.group
	if groupName == "" {
		groupName = "default"
	}

	repos := cfg.GetRepos(groupName)

	if len(repos) == 0 {
		fmt.Printf("No repositories found for group '%s'.\n", groupName)
		return
	}

	fmt.Printf("Repositories for group '%s':\n", groupName)
	for _, repo := range repos {
		if repo.Path != "" {
			fmt.Printf("  - %s: %s\n", repo.Name, repo.Path)
		} else if repo.URL != "" {
			fmt.Printf("  - %s: %s (remote)\n", repo.Name, repo.URL)
		} else {
			fmt.Printf("  - %s: (unknown location)\n", repo.Name)
		}
	}
}