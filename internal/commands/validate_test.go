package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateCommandRequiresSkillOrAll(t *testing.T) {
	cmd := newValidateCmd()
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want arg validation error")
	}
	if !strings.Contains(err.Error(), "provide exactly one skill name") {
		t.Fatalf("Execute() error = %q, want arg validation message", err)
	}
}

func TestValidateCommandRejectsArgWithAll(t *testing.T) {
	cmd := newValidateCmd()
	cmd.SetArgs([]string{"go", "--all"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want invalid flag/arg combination")
	}
	if !strings.Contains(err.Error(), "--all validates every skill and cannot be combined") {
		t.Fatalf("Execute() error = %q, want --all conflict message", err)
	}
}

func TestValidateCommandSingleSkillSuccess(t *testing.T) {
	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "project")
	xdgConfig := filepath.Join(tmp, "xdg")
	globalSkill := filepath.Join(xdgConfig, "bond", "lang", "go")

	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(projectRoot) error = %v", err)
	}
	if err := os.MkdirAll(globalSkill, 0o755); err != nil {
		t.Fatalf("MkdirAll(globalSkill) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(globalSkill, "SKILL.md"), []byte("---\nname: go\ndescription: Go skill\n---\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(SKILL.md) error = %v", err)
	}

	prevWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}
	t.Cleanup(func() {
		if chdirErr := os.Chdir(prevWd); chdirErr != nil {
			t.Fatalf("restore cwd error = %v", chdirErr)
		}
	})
	if err := os.Chdir(projectRoot); err != nil {
		t.Fatalf("os.Chdir() error = %v", err)
	}

	t.Setenv("XDG_CONFIG_HOME", xdgConfig)

	buf := &bytes.Buffer{}
	cmd := newValidateCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"go"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "[OK] go") {
		t.Fatalf("output missing success line: %q", output)
	}
}

func TestValidateCommandSingleSkillInvalidReturnsError(t *testing.T) {
	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "project")
	xdgConfig := filepath.Join(tmp, "xdg")
	globalSkill := filepath.Join(xdgConfig, "bond", "lang", "go")

	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(projectRoot) error = %v", err)
	}
	if err := os.MkdirAll(globalSkill, 0o755); err != nil {
		t.Fatalf("MkdirAll(globalSkill) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(globalSkill, "SKILL.md"), []byte("---\nname: Go\ndescription: \"\"\n---\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(SKILL.md) error = %v", err)
	}

	prevWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}
	t.Cleanup(func() {
		if chdirErr := os.Chdir(prevWd); chdirErr != nil {
			t.Fatalf("restore cwd error = %v", chdirErr)
		}
	})
	if err := os.Chdir(projectRoot); err != nil {
		t.Fatalf("os.Chdir() error = %v", err)
	}

	t.Setenv("XDG_CONFIG_HOME", xdgConfig)

	buf := &bytes.Buffer{}
	cmd := newValidateCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"go"})

	err = cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want validation failure")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Fatalf("Execute() error = %q, want validation failure", err)
	}

	output := buf.String()
	if !strings.Contains(output, "[ERROR] go") {
		t.Fatalf("output missing invalid line: %q", output)
	}
}

func TestValidateCommandAllSkillsMixedResults(t *testing.T) {
	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "project")
	xdgConfig := filepath.Join(tmp, "xdg")
	goSkill := filepath.Join(xdgConfig, "bond", "lang", "go")
	rustSkill := filepath.Join(xdgConfig, "bond", "systems", "rust")

	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(projectRoot) error = %v", err)
	}
	if err := os.MkdirAll(goSkill, 0o755); err != nil {
		t.Fatalf("MkdirAll(goSkill) error = %v", err)
	}
	if err := os.MkdirAll(rustSkill, 0o755); err != nil {
		t.Fatalf("MkdirAll(rustSkill) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(goSkill, "SKILL.md"), []byte("---\nname: go\ndescription: Go skill\n---\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(go/SKILL.md) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(rustSkill, "SKILL.md"), []byte("---\nname: rust\ndescription: \"\"\n---\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(rust/SKILL.md) error = %v", err)
	}

	prevWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}
	t.Cleanup(func() {
		if chdirErr := os.Chdir(prevWd); chdirErr != nil {
			t.Fatalf("restore cwd error = %v", chdirErr)
		}
	})
	if err := os.Chdir(projectRoot); err != nil {
		t.Fatalf("os.Chdir() error = %v", err)
	}

	t.Setenv("XDG_CONFIG_HOME", xdgConfig)

	buf := &bytes.Buffer{}
	cmd := newValidateCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--all"})

	err = cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want mixed validation failure")
	}

	output := buf.String()
	if !strings.Contains(output, "[OK] go") {
		t.Fatalf("output missing go success: %q", output)
	}
	if !strings.Contains(output, "[ERROR] rust") {
		t.Fatalf("output missing rust invalid: %q", output)
	}
}
