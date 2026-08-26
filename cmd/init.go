package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const defaultYAML = `# testwarden configuration
coverage:
  unit_threshold: 80
  integration_gap_threshold: 5
  unit_path: coverage.out
  integration_path: integration-coverage.out
  unit_command: "go test -coverprofile=coverage.out ./..."
  integration_command: "go test -tags=integration -coverprofile=integration-coverage.out ./..."

mocks:
  detect_overmocking: true
  real_dependencies:
    go:
      - database/sql
      - net/http
      - os
    typescript:
      - fs
      - http
      - pg
      - mysql2

ai:
  endpoint: "http://localhost:11434/v1"
  api_key: ""
  model: "qwen2.5-coder"
  timeout: 120
  max_tokens: 4096

languages: [go, typescript]
`

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create .testwarden.yml with defaults in cwd",
	RunE: func(cmd *cobra.Command, args []string) error {
		target := ".testwarden.yml"
		if _, err := os.Stat(target); err == nil {
			return fmt.Errorf("%s already exists", target)
		}
		if err := os.WriteFile(target, []byte(defaultYAML), 0o644); err != nil {
			return err
		}
		fmt.Printf("✓ created %s\n", target)
		return nil
	},
}
