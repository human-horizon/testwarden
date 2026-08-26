// Package cmd implements the testwarden CLI.
package cmd

import (
	"io"
	"os"

	"github.com/spf13/cobra"
)

var (
	flagConfig string
	flagLang   string
	flagJSON   bool
	flagDryRun bool
	flagNoTUI  bool
)

// rootCmd is the entrypoint command.
var rootCmd = &cobra.Command{
	Use:   "testwarden",
	Short: "Test coverage and mock quality watchdog",
	Long: `testwarden analyses Go and TypeScript projects to detect:
- Unit test coverage below threshold
- Over-mocking in unit tests
- Coverage gaps between unit and integration tests
It can optionally auto-fix issues via a local OpenAI-compatible LLM.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagConfig, "config", ".testwarden.yml", "path to config file")
	rootCmd.PersistentFlags().StringVar(&flagLang, "lang", "", "limit to language: go|typescript")
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "output JSON report")
	rootCmd.PersistentFlags().BoolVar(&flagNoTUI, "no-tui", false, "disable interactive TUI (use plain text output)")

	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(fixCmd)
}

func defaultOut() io.Writer {
	return os.Stdout
}
