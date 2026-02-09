package commands

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

const (
	levelInfo  = "INFO"
	levelOK    = "OK"
	levelWarn  = "WARN"
	levelError = "ERROR"
)

func writeLevelLine(w io.Writer, level string, format string, args ...any) error {
	msg := fmt.Sprintf(format, args...)
	_, err := fmt.Fprintf(w, "[%s] %s\n", level, msg)
	return err
}

func printOut(cmd *cobra.Command, level string, format string, args ...any) error {
	return writeLevelLine(cmd.OutOrStdout(), level, format, args...)
}

func printErr(cmd *cobra.Command, level string, format string, args ...any) error {
	return writeLevelLine(cmd.ErrOrStderr(), level, format, args...)
}

// PrintRootError writes a top-level CLI error using the shared output format.
func PrintRootError(w io.Writer, err error) error {
	return writeLevelLine(w, levelError, "%v", err)
}
