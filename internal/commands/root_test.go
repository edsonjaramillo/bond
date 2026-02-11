package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withRootOutputShowLevel(t *testing.T, show bool) {
	t.Helper()
	prev := outputShowLevel
	setOutputShowLevel(show)
	t.Cleanup(func() {
		setOutputShowLevel(prev)
	})
}

func TestRootRejectsInvalidColorFlagValue(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--color=invalid", "status"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), `invalid value for --color: "invalid"`) {
		t.Fatalf("error = %q, want invalid --color message", err.Error())
	}
}

func TestRootRegistersCopyCommand(t *testing.T) {
	cmd := newRootCmd()

	found := false
	for _, child := range cmd.Commands() {
		if child.Name() == "copy" {
			found = true
			break
		}
	}

	if !found {
		t.Fatal("newRootCmd() missing copy command")
	}
}

func TestRootRegistersStoreCommand(t *testing.T) {
	cmd := newRootCmd()

	found := false
	for _, child := range cmd.Commands() {
		if child.Name() == "store" {
			found = true
			break
		}
	}

	if !found {
		t.Fatal("newRootCmd() missing store command")
	}
}

func TestRootRegistersCreateCommand(t *testing.T) {
	cmd := newRootCmd()

	found := false
	for _, child := range cmd.Commands() {
		if child.Name() == "create" {
			found = true
			break
		}
	}

	if !found {
		t.Fatal("newRootCmd() missing create command")
	}
}

func TestRootRegistersEditCommand(t *testing.T) {
	cmd := newRootCmd()

	found := false
	for _, child := range cmd.Commands() {
		if child.Name() == "edit" {
			found = true
			break
		}
	}

	if !found {
		t.Fatal("newRootCmd() missing edit command")
	}
}

func TestRootNoLevelFlagAppliesToSubcommands(t *testing.T) {
	withRootOutputShowLevel(t, true)

	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "project")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(projectRoot) error = %v", err)
	}
	chdirForTest(t, projectRoot)

	buf := &bytes.Buffer{}
	cmd := newRootCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--no-level", "init"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if got, want := buf.String(), "initialized .agents/skills\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestRootNoLevelFromEnvAppliesToSubcommands(t *testing.T) {
	withRootOutputShowLevel(t, true)
	t.Setenv(envNoLevel, "true")

	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "project")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(projectRoot) error = %v", err)
	}
	chdirForTest(t, projectRoot)

	buf := &bytes.Buffer{}
	cmd := newRootCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"init"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if got, want := buf.String(), "initialized .agents/skills\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestRootNoLevelFlagFalseOverridesEnvTrue(t *testing.T) {
	withRootOutputShowLevel(t, true)
	t.Setenv(envNoLevel, "true")

	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "project")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(projectRoot) error = %v", err)
	}
	chdirForTest(t, projectRoot)

	buf := &bytes.Buffer{}
	cmd := newRootCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--no-level=false", "init"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if got, want := buf.String(), "[OK] initialized .agents/skills\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestRootNoLevelFlagTrueOverridesEnvFalse(t *testing.T) {
	withRootOutputShowLevel(t, true)
	t.Setenv(envNoLevel, "false")

	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "project")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(projectRoot) error = %v", err)
	}
	chdirForTest(t, projectRoot)

	buf := &bytes.Buffer{}
	cmd := newRootCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--no-level=true", "init"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if got, want := buf.String(), "initialized .agents/skills\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestRootRejectsInvalidNoLevelEnvValue(t *testing.T) {
	t.Setenv(envNoLevel, "maybe")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"status"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), `invalid value for BOND_NO_LEVEL: "maybe"`) {
		t.Fatalf("error = %q, want invalid BOND_NO_LEVEL message", err.Error())
	}
}
