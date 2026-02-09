package main

import (
	"os"

	"bond/internal/commands"
)

// main runs the CLI and exits with a non-zero status on command errors.
func main() {
	if err := commands.Execute(); err != nil {
		if !commands.IsAlreadyReportedFailure(err) {
			_ = commands.PrintRootError(os.Stderr, err)
		}
		os.Exit(1)
	}
}
