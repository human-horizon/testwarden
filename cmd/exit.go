package cmd

import "os"

// osExit is overridable for tests.
var osExit = os.Exit
