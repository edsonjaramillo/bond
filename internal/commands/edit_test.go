package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditCommandRequiresExactlyOneSkillNameArg(t *testing.T) {
	cmd := newEditCmd()
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want arg validation error")
	}
	if !strings.Contains(err.Error(), "accepts 1 arg(s), received 0") {
		t.Fatalf("Execute() error = %q, want exact-args error", err)
	}
}

func TestEditCommandReturnsErrorWhenEditorIsNotSet(t *testing.T) {
	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "project")
	xdgConfig := filepath.Join(tmp, "xdg")
	skillDir := filepath.Join(xdgConfig, "bond", "go")

	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(projectRoot) error = %v", err)
	}
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(skillDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(SKILL.md) error = %v", err)
	}

	chdirForTest(t, projectRoot)
	t.Setenv("XDG_CONFIG_HOME", xdgConfig)
	t.Setenv("EDITOR", "")

	cmd := newEditCmd()
	cmd.SetArgs([]string{"go"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "EDITOR environment variable is not set") {
		t.Fatalf("Execute() error = %q, want missing EDITOR error", err)
	}
}

func TestEditCommandReturnsErrorWhenNoMatchingSkill(t *testing.T) {
	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "project")
	xdgConfig := filepath.Join(tmp, "xdg")

	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(projectRoot) error = %v", err)
	}

	chdirForTest(t, projectRoot)
	t.Setenv("XDG_CONFIG_HOME", xdgConfig)
	t.Setenv("EDITOR", "true")

	cmd := newEditCmd()
	cmd.SetArgs([]string{"missing"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "no matching skills: missing") {
		t.Fatalf("Execute() error = %q, want no matching skills error", err)
	}
}

func TestEditCommandOpensSkillWithEditor(t *testing.T) {
	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "project")
	xdgConfig := filepath.Join(tmp, "xdg")
	skillDir := filepath.Join(xdgConfig, "bond", "go")

	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(projectRoot) error = %v", err)
	}
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(skillDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(SKILL.md) error = %v", err)
	}

	chdirForTest(t, projectRoot)
	t.Setenv("XDG_CONFIG_HOME", xdgConfig)
	t.Setenv("EDITOR", "true")

	buf := &bytes.Buffer{}
	cmd := newEditCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"go"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestEditCommandReturnsEditorFailure(t *testing.T) {
	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "project")
	xdgConfig := filepath.Join(tmp, "xdg")
	skillDir := filepath.Join(xdgConfig, "bond", "go")

	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(projectRoot) error = %v", err)
	}
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(skillDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(SKILL.md) error = %v", err)
	}

	chdirForTest(t, projectRoot)
	t.Setenv("XDG_CONFIG_HOME", xdgConfig)
	t.Setenv("EDITOR", "false")

	cmd := newEditCmd()
	cmd.SetArgs([]string{"go"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), `failed to open editor for "go"`) {
		t.Fatalf("Execute() error = %q, want editor failure prefix", err)
	}
}
