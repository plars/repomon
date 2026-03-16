package main

import (
	"fmt"

	"github.com/plars/repomon/internal/config"
	"github.com/spf13/cobra"
)

func (r *repomonRunner) versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version number of repomon",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(r.output, "repomon version %s\n", config.Version)
		},
	}
}
