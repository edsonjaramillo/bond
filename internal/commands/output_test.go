package commands

import (
	"bytes"
	"errors"
	"testing"

	"bond/internal/skills"
	"github.com/spf13/cobra"
)

func withOutputColorMode(t *testing.T, mode string) {
	t.Helper()
	prev := outputColorMode
	setOutputColorMode(mode)
	t.Cleanup(func() {
		setOutputColorMode(prev)
	})
}

func withOutputShowLevel(t *testing.T, show bool) {
	t.Helper()
	prev := outputShowLevel
	setOutputShowLevel(show)
	t.Cleanup(func() {
		setOutputShowLevel(prev)
	})
}

func TestPrintOutPrefixesTagAndWritesStdout(t *testing.T) {
	withOutputColorMode(t, colorModeNever)
	withOutputShowLevel(t, true)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	if err := printOut(cmd, levelOK, "linked %s", "go"); err != nil {
		t.Fatalf("printOut() error = %v", err)
	}

	if got, want := stdout.String(), "[OK] linked go\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("stderr = %q, want empty", got)
	}
}

func TestPrintErrPrefixesTagAndWritesStderr(t *testing.T) {
	withOutputColorMode(t, colorModeNever)
	withOutputShowLevel(t, true)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	if err := printErr(cmd, levelError, "failed %s", "go"); err != nil {
		t.Fatalf("printErr() error = %v", err)
	}

	if got := stdout.String(); got != "" {
		t.Fatalf("stdout = %q, want empty", got)
	}
	if got, want := stderr.String(), "[ERROR] failed go\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestStatusLevelMapping(t *testing.T) {
	tests := []struct {
		name   string
		input  skills.StatusKind
		expect string
	}{
		{name: "linked", input: skills.StatusLinked, expect: levelOK},
		{name: "broken", input: skills.StatusBroken, expect: levelError},
		{name: "external", input: skills.StatusExternal, expect: levelWarn},
		{name: "unknown", input: skills.StatusKind("mystery"), expect: levelInfo},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := statusLevel(tc.input)
			if got != tc.expect {
				t.Fatalf("statusLevel(%q) = %q, want %q", tc.input, got, tc.expect)
			}
		})
	}
}

func TestPrintRootErrorUsesSharedErrorFormat(t *testing.T) {
	withOutputColorMode(t, colorModeNever)
	withOutputShowLevel(t, true)

	stderr := &bytes.Buffer{}
	if err := PrintRootError(stderr, errors.New("boom")); err != nil {
		t.Fatalf("PrintRootError() error = %v", err)
	}
	if got, want := stderr.String(), "[ERROR] boom\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestParseColorMode(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "auto", input: "auto", want: colorModeAuto},
		{name: "always", input: "always", want: colorModeAlways},
		{name: "never", input: "never", want: colorModeNever},
		{name: "trim and case-insensitive", input: "  AlWaYs ", want: colorModeAlways},
		{name: "invalid", input: "sometimes", wantErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseColorMode(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseColorMode(%q) error = nil, want non-nil", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseColorMode(%q) error = %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("parseColorMode(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestPrintOutAlwaysColorsLevel(t *testing.T) {
	withOutputColorMode(t, colorModeAlways)
	withOutputShowLevel(t, true)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	if err := printOut(cmd, levelOK, "linked %s", "go"); err != nil {
		t.Fatalf("printOut() error = %v", err)
	}

	if got, want := stdout.String(), "["+ansiGreen+"OK"+ansiReset+"] linked go\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestPrintOutAlwaysIgnoresNoColor(t *testing.T) {
	withOutputColorMode(t, colorModeAlways)
	withOutputShowLevel(t, true)
	t.Setenv("NO_COLOR", "1")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	if err := printOut(cmd, levelError, "failed %s", "go"); err != nil {
		t.Fatalf("printOut() error = %v", err)
	}

	if got, want := stdout.String(), "["+ansiRed+"ERROR"+ansiReset+"] failed go\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestPrintOutAutoHonorsNoColor(t *testing.T) {
	withOutputColorMode(t, colorModeAuto)
	withOutputShowLevel(t, true)
	t.Setenv("NO_COLOR", "1")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	if err := printOut(cmd, levelWarn, "conflict %s", "go"); err != nil {
		t.Fatalf("printOut() error = %v", err)
	}

	if got, want := stdout.String(), "[WARN] conflict go\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestPrintOutWithoutLevelTag(t *testing.T) {
	withOutputColorMode(t, colorModeAlways)
	withOutputShowLevel(t, false)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	if err := printOut(cmd, levelWarn, "conflict %s", "go"); err != nil {
		t.Fatalf("printOut() error = %v", err)
	}

	if got, want := stdout.String(), "conflict go\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestPrintErrWithoutLevelTag(t *testing.T) {
	withOutputColorMode(t, colorModeAlways)
	withOutputShowLevel(t, false)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	if err := printErr(cmd, levelError, "failed %s", "go"); err != nil {
		t.Fatalf("printErr() error = %v", err)
	}

	if got, want := stderr.String(), "failed go\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestPrintRootErrorWithoutLevelTag(t *testing.T) {
	withOutputColorMode(t, colorModeNever)
	withOutputShowLevel(t, false)

	stderr := &bytes.Buffer{}
	if err := PrintRootError(stderr, errors.New("boom")); err != nil {
		t.Fatalf("PrintRootError() error = %v", err)
	}
	if got, want := stderr.String(), "boom\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}
