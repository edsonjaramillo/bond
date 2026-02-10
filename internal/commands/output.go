package commands

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const (
	levelInfo  = "INFO"
	levelOK    = "OK"
	levelWarn  = "WARN"
	levelError = "ERROR"
)

const (
	colorModeAuto   = "auto"
	colorModeAlways = "always"
	colorModeNever  = "never"
)

const (
	ansiReset  = "\x1b[0m"
	ansiRed    = "\x1b[31m"
	ansiYellow = "\x1b[33m"
	ansiCyan   = "\x1b[36m"
	ansiGreen  = "\x1b[32m"
)

var (
	outputColorMode = colorModeAuto
	outputShowLevel = true
)

func parseColorMode(raw string) (string, error) {
	mode := strings.ToLower(strings.TrimSpace(raw))
	switch mode {
	case colorModeAuto, colorModeAlways, colorModeNever:
		return mode, nil
	default:
		return "", fmt.Errorf("invalid value for --color: %q (want auto, always, or never)", raw)
	}
}

func setOutputColorMode(mode string) {
	outputColorMode = mode
}

func setOutputShowLevel(show bool) {
	outputShowLevel = show
}

func shouldColorize(w io.Writer, mode string) bool {
	switch mode {
	case colorModeAlways:
		return true
	case colorModeNever:
		return false
	case colorModeAuto:
		if _, disabled := os.LookupEnv("NO_COLOR"); disabled {
			return false
		}
		return isTTY(w)
	default:
		return false
	}
}

func isTTY(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func levelColor(level string) string {
	switch level {
	case levelError:
		return ansiRed
	case levelWarn:
		return ansiYellow
	case levelInfo:
		return ansiCyan
	case levelOK:
		return ansiGreen
	default:
		return ""
	}
}

func formatLevel(level string, colorEnabled bool) string {
	if !colorEnabled {
		return level
	}
	color := levelColor(level)
	if color == "" {
		return level
	}
	return color + level + ansiReset
}

func writeLevelLine(w io.Writer, level string, format string, args ...any) error {
	msg := fmt.Sprintf(format, args...)
	if !outputShowLevel {
		_, err := fmt.Fprintf(w, "%s\n", msg)
		return err
	}
	tag := formatLevel(level, shouldColorize(w, outputColorMode))
	_, err := fmt.Fprintf(w, "[%s] %s\n", tag, msg)
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
