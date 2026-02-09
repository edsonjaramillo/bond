package main

import (
	"fmt"
	"os"

	"bond/internal/commands"
)

// main runs the CLI and exits with a non-zero status on command errors.
func main() {
	if err := commands.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
		os.Exit(1)
	}
}
