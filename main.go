// Command testwarden is a CLI watchdog for test coverage and over-mocking.
package main

import (
	"fmt"
	"os"

	"github.com/HumanHorizon/testwarden/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
