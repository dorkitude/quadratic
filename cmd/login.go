package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(loginCmd)
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Foursquare",
	RunE: func(cmd *cobra.Command, _ []string) error {
		a := mustApp(cmd)
		ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
		defer cancel()

		token, err := a.Login(ctx)
		if err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Saved access token. Length=%d\n", len(token))
		return nil
	},
}
