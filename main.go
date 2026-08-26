package main

import (
	"context"
	"fmt"
	"os"

	"github.com/edsonjaramillo/bond/internal/application"
)

func main() {
	os.Exit(execute())
}

func execute() int {
	workingDirectory, err := os.Getwd()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)

		return 1
	}

	return application.Run(context.Background(), application.Invocation{
		Arguments:        os.Args[1:],
		Environment:      os.Environ(),
		WorkingDirectory: workingDirectory,
		Stdin:            os.Stdin,
		Stdout:           os.Stdout,
		Stderr:           os.Stderr,
	}, application.Dependencies{Version: application.Version})
}
