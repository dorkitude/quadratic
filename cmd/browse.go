package cmd

import (
	"context"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(browseCmd)
}

var browseCmd = &cobra.Command{
	Use:   "browse",
	Short: "Browse local check-in backups in a web UI",
	RunE: func(cmd *cobra.Command, _ []string) error {
		a := mustApp(cmd)
		ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Minute)
		defer cancel()
		return a.Browse(ctx)
	},
}
