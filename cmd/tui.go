package cmd

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(tuiCmd)
}

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Open the terminal dashboard",
	RunE: func(cmd *cobra.Command, _ []string) error {
		a := mustApp(cmd)
		model, err := a.NewModel(context.Background())
		if err != nil {
			return err
		}

		program := tea.NewProgram(model)
		_, err = program.Run()
		return err
	},
}
