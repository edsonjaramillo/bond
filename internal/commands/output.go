package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

const (
	levelInfo  = "INFO"
	levelOK    = "OK"
	levelWarn  = "WARN"
	levelError = "ERROR"
)

func printOut(cmd *cobra.Command, level string, format string, args ...any) error {
	msg := fmt.Sprintf(format, args...)
	_, err := fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s\n", level, msg)
	return err
}

func printErr(cmd *cobra.Command, level string, format string, args ...any) error {
	msg := fmt.Sprintf(format, args...)
	_, err := fmt.Fprintf(cmd.ErrOrStderr(), "[%s] %s\n", level, msg)
	return err
}
