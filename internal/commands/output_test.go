package commands

import (
	"bytes"
	"errors"
	"testing"

	"bond/internal/skills"
	"github.com/spf13/cobra"
)

func TestPrintOutPrefixesTagAndWritesStdout(t *testing.T) {
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
	stderr := &bytes.Buffer{}
	if err := PrintRootError(stderr, errors.New("boom")); err != nil {
		t.Fatalf("PrintRootError() error = %v", err)
	}
	if got, want := stderr.String(), "[ERROR] boom\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}
