package cmd

import (
	"fmt"
	"os"

	"quadratic/internal/app"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "quadratic",
	Short:         "Back up Foursquare check-ins locally",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func mustApp(cmd *cobra.Command) *app.App {
	instance, err := app.New()
	if err != nil {
		cobra.CheckErr(err)
	}
	return instance
}
