package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/HumanHorizon/testwarden/internal/config"
	"github.com/HumanHorizon/testwarden/internal/runner"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Analyse project and report issues (exit 1 if any)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(flagConfig)
		if err != nil {
			return err
		}
		if flagLang != "" {
			cfg.Languages = []string{flagLang}
		}

		code, err := runner.RunCheck(context.Background(), runner.Options{
			Cfg:   cfg,
			Root:  ".",
			JSON:  flagJSON,
			TUI:   false, // check never uses TUI
			Quiet: flagQuiet,
			Out:   defaultOut(),
		})
		if err != nil {
			return err
		}

		if code != 0 {
			// cobra prints errors itself; we just signal non-zero exit
			osExit(1)
		}
		return nil
	},
}
