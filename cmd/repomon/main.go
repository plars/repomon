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

func main() {
	var configFile string
	var days int
	var group string

	var rootCmd = &cobra.Command{
		Use:   "repomon",
		Short: "A tool to monitor git repositories and report recent changes",
		Long: `Repomon monitors configured git repositories and generates a report
showing the most recent commits to each repository in an easy-to-read format.`,
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.Load(configFile)
			if err != nil {
				slog.Error("Failed to load configuration", "error", err)
				os.Exit(1)
			}

			if cmd.Flags().Changed("days") {
				cfg.Defaults.Days = days
			}

			debugEnabled, _ := cmd.Flags().GetBool("debug")
			if debugEnabled {
				slog.SetLogLoggerLevel(slog.LevelDebug)
			}

			groupName := group
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
		},
	}

	rootCmd.Flags().StringVarP(&configFile, "config", "c", "", "path to config file (default ~/.config/repomon/config.toml)")
	rootCmd.Flags().IntVarP(&days, "days", "d", 1, "number of days to look back in history")
	rootCmd.Flags().StringVarP(&group, "group", "g", "", "repository group to use (default: 'default')")
	rootCmd.Flags().Bool("debug", false, "enable debug logging")

	if err := rootCmd.Execute(); err != nil {
		slog.Error("Command execution failed", "error", err)
		os.Exit(1)
	}
}
