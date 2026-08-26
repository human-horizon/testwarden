package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/HumanHorizon/testwarden/internal/config"
	"github.com/HumanHorizon/testwarden/internal/runner"
)

var fixCmd = &cobra.Command{
	Use:   "fix",
	Short: "Analyse project and auto-fix issues via local AI",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(flagConfig)
		if err != nil {
			return err
		}
		if flagLang != "" {
			cfg.Languages = []string{flagLang}
		}

		code, err := runner.RunFix(context.Background(), runner.Options{
			Cfg:    cfg,
			Root:   ".",
			JSON:   flagJSON,
			DryRun: flagDryRun,
			TUI:    flagTUI,
			Quiet:  flagQuiet,
			Out:    defaultOut(),
		})
		if err != nil {
			return err
		}

		if code != 0 {
			osExit(1)
		}
		return nil
	},
}

func init() {
	fixCmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "show what would be fixed without writing")
}
