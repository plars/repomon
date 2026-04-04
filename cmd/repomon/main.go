package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/plars/repomon/internal/config"
	"github.com/plars/repomon/internal/git"
	"github.com/plars/repomon/internal/report"
	"github.com/spf13/cobra"
)

// rootOptions holds the persistent flags common to all commands.
type rootOptions struct {
	configFile string
	group      string
	version    bool
}

// runOptions holds the flags specific to the 'run' command.
type runOptions struct {
	days  int
	debug bool
}

// GitMonitor defines the interface for monitoring git repositories.
type GitMonitor interface {
	GetRecentCommits(ctx context.Context) ([]git.RepoResult, error)
	SetDays(days int)
}

// ReportFormatter defines the interface for formatting reports.
type ReportFormatter interface {
	Format(results []git.RepoResult) (string, error)
}

// repomonRunner handles the execution of repomon commands.
type repomonRunner struct {
	output io.Writer
	err    io.Writer
	stdin  io.Reader

	// Dependency injection for testing
	loadConfig    func(string) (*config.Config, error)
	newGitMonitor func([]config.Repo) GitMonitor
	newFormatter  func() ReportFormatter
}

func newDefaultRunner(out, err io.Writer, stdin io.Reader) *repomonRunner {
	return &repomonRunner{
		output:     out,
		err:        err,
		stdin:      stdin,
		loadConfig: config.Load,
		newGitMonitor: func(repos []config.Repo) GitMonitor {
			return git.NewMonitorWithRepos(repos)
		},
		newFormatter: func() ReportFormatter {
			return report.NewFormatter()
		},
	}
}

// resolveConfigPath returns the full path to the config file.
// If configFile is empty, it returns the default path (~/.config/repomon/config.yaml).
// Environment variables in the path are expanded.
func resolveConfigPath(configFile string) (string, error) {
	if configFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		return filepath.Join(home, ".config", "repomon", "config.yaml"), nil
	}
	return os.ExpandEnv(configFile), nil
}

func main() {
	// Initialize option structs
	rootOpts := &rootOptions{}
	runOpts := &runOptions{}
	runner := newDefaultRunner(os.Stdout, os.Stderr, os.Stdin)

	var rootCmd = &cobra.Command{
		Use:   "repomon",
		Short: "A tool to monitor git repositories and report recent changes",
		Long: `Repomon monitors configured git repositories and generates a report
showing the most recent commits to each repository in an easy-to-read format.`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := runner.executeRun(cmd.Context(), args, runOpts, rootOpts); err != nil {
				slog.Error("Run command failed", "error", err)
				os.Exit(1)
			}
		},
	}

	// Bind persistent flags to rootOptions
	rootCmd.PersistentFlags().StringVarP(&rootOpts.configFile, "config", "c", "", "path to config file (default ~/.config/repomon/config.yaml)")
	rootCmd.PersistentFlags().StringVarP(&rootOpts.group, "group", "g", "", "repository group to use (default: 'default')")
	// Bind run-specific flags to runOptions
	rootCmd.Flags().IntVarP(&runOpts.days, "days", "d", 1, "number of days to look back in history")
	rootCmd.Flags().BoolVar(&runOpts.debug, "debug", false, "enable debug logging")

	versionCmd := runner.versionCmd()

	// Add a persistent --version flag that just calls the version command
	rootCmd.PersistentFlags().BoolVarP(&rootOpts.version, "version", "v", false, "print the version number")

	// If --version is set, call the version command and exit
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if rootOpts.version {
			versionCmd.Run(cmd, args)
			os.Exit(0)
		}
		return nil
	}

	rootCmd.AddCommand(runner.listCmd(rootOpts))
	rootCmd.AddCommand(versionCmd)

	rootCmd.AddCommand(runner.addCmd(rootOpts))

	rootCmd.AddCommand(runner.rmCmd(rootOpts))

	if err := rootCmd.Execute(); err != nil {
		slog.Error("Command execution failed", "error", err)
		os.Exit(1)
	}
}

// executeRun contains the core logic for the 'run' command.
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
