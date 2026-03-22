package cmd

import (
	"context"
	"encoding/json"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(syncCmd)
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Backfill and persist check-ins locally",
	RunE: func(cmd *cobra.Command, _ []string) error {
		a := mustApp(cmd)
		ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Minute)
		defer cancel()

		result, err := a.Sync(ctx)
		if err != nil {
			return err
		}

		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	},
}
