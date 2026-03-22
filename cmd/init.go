package cmd

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"quadratic/internal/config"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(initCmd)
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create the local config file interactively",
	RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		if cfg.ConfigPath == "" || cfg.DataDir == "" {
			configPath, dataDir, err := config.DefaultPaths()
			if err != nil {
				return err
			}
			cfg.ConfigPath = configPath
			if cfg.DataDir == "" {
				cfg.DataDir = dataDir
			}
		}

		reader := bufio.NewReader(cmd.InOrStdin())
		out := cmd.OutOrStdout()

		fmt.Fprintf(out, "Config path: %s\n", cfg.ConfigPath)
		fmt.Fprintf(out, "Press enter to keep the default shown in brackets.\n\n")

		if value, err := prompt(reader, out, "Client ID", cfg.ClientID, false); err != nil {
			return err
		} else {
			cfg.ClientID = value
		}

		if value, err := prompt(reader, out, "Client Secret", cfg.ClientSecret, false); err != nil {
			return err
		} else {
			cfg.ClientSecret = value
		}

		if value, err := prompt(reader, out, "Redirect URL", cfg.RedirectURL, true); err != nil {
			return err
		} else {
			cfg.RedirectURL = value
		}

		if value, err := prompt(reader, out, "Data directory", cfg.DataDir, true); err != nil {
			return err
		} else {
			cfg.DataDir = value
		}

		if cfg.ClientID == "" || cfg.ClientSecret == "" {
			return fmt.Errorf("client id and client secret are required")
		}

		if err := config.Save(cfg); err != nil {
			return err
		}

		fmt.Fprintf(out, "\nSaved config to %s\n", cfg.ConfigPath)
		fmt.Fprintf(out, "Next: run `quadratic login`\n")
		return nil
	},
}

func prompt(reader *bufio.Reader, out io.Writer, label string, current string, allowEmptyDefault bool) (string, error) {
	display := current
	if display == "" && !allowEmptyDefault {
		display = ""
	}

	if display != "" {
		fmt.Fprintf(out, "%s [%s]: ", label, display)
	} else {
		fmt.Fprintf(out, "%s: ", label)
	}

	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	value := strings.TrimSpace(line)
	if value == "" {
		return current, nil
	}
	return value, nil
}
