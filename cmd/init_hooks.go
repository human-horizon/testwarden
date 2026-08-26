package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/HumanHorizon/testwarden/internal/hooks"
)

var initHooksCmd = &cobra.Command{
	Use:   "init-hooks",
	Short: "Install testwarden as a git pre-commit hook",
	Long: `Installs testwarden as a git pre-commit hook in the current project.
The hook runs 'testwarden check' before each commit and aborts if issues are found.
Run 'testwarden uninstall-hooks' to remove.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		hookPath, err := hooks.Install(".")
		if err != nil {
			return err
		}
		fmt.Printf("✓ installed pre-commit hook at %s\n", hookPath)
		return nil
	},
}

var uninstallHooksCmd = &cobra.Command{
	Use:   "uninstall-hooks",
	Short: "Remove testwarden git pre-commit hook",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := hooks.Uninstall("."); err != nil {
			return err
		}
		fmt.Println("✓ removed pre-commit hook")
		return nil
	},
}
