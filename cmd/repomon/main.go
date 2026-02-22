package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

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

// rmOptions holds the flags specific to the 'rm' command.
type rmOptions struct {
	force bool
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

	// Dependency injection for testing
	loadConfig    func(string) (*config.Config, error)
	newGitMonitor func([]config.Repo) GitMonitor
	newFormatter  func() ReportFormatter
}

func newDefaultRunner(out, err io.Writer) *repomonRunner {
	return &repomonRunner{
		output:     out,
		err:        err,
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
	rmOpts := &rmOptions{}
	runner := newDefaultRunner(os.Stdout, os.Stderr)

	var runCmd = &cobra.Command{
		Use:   "run",
		Short: "Monitors configured git repositories and reports recent changes",
		Run: func(cmd *cobra.Command, args []string) {
			if err := runner.executeRun(cmd.Context(), args, runOpts, rootOpts); err != nil {
				slog.Error("Run command failed", "error", err)
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
	rootCmd.PersistentFlags().StringVarP(&rootOpts.configFile, "config", "c", "", "path to config file (default ~/.config/repomon/config.yaml)")
	rootCmd.PersistentFlags().StringVarP(&rootOpts.group, "group", "g", "", "repository group to use (default: 'default')")
	// Add run-specific flags to rootCmd so they work without 'run' subcommand
	rootCmd.Flags().AddFlagSet(runCmd.Flags())

	var listCmd = &cobra.Command{
		Use:   "list",
		Short: "Lists configured git repositories",
		Run: func(cmd *cobra.Command, args []string) {
			if err := runner.executeList(args, rootOpts); err != nil {
				slog.Error("List command failed", "error", err)
				os.Exit(1)
			}
		},
	}

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(listCmd)

	var addCmd = &cobra.Command{
		Use:   "add <repo>",
		Short: "Adds a repository to the configuration",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := runner.executeAdd(args, rootOpts); err != nil {
				slog.Error("Add command failed", "error", err)
				os.Exit(1)
			}
		},
	}

	rootCmd.AddCommand(addCmd)

	var rmCmd = &cobra.Command{
		Use:   "rm <repo>",
		Short: "Removes a repository from the configuration",
		Long: `Removes a repository from the configuration. The repository can be
identified by its short name (as shown in 'list') or by its full path/URL.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := runner.executeRm(args, rootOpts, rmOpts); err != nil {
				slog.Error("Remove command failed", "error", err)
				os.Exit(1)
			}
		},
	}
	rmCmd.Flags().BoolVarP(&rmOpts.force, "force", "f", false, "skip confirmation prompt")

	rootCmd.AddCommand(rmCmd)

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
		reader := bufio.NewReader(os.Stdin)
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
