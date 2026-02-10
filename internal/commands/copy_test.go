package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCopyCommandRequiresAtLeastOneSkillArg(t *testing.T) {
	cmd := newCopyCmd()
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want arg validation error")
	}
	if !strings.Contains(err.Error(), "requires at least 1 arg(s)") {
		t.Fatalf("Execute() error = %q, want minimum args error", err)
	}
}

func TestCopyCommandCopiesAndThenSkipsExisting(t *testing.T) {
	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "project")
	xdgConfig := filepath.Join(tmp, "xdg")
	globalSkill := filepath.Join(xdgConfig, "bond", "lang", "go")
	projectSkill := filepath.Join(projectRoot, ".agents", "skills", "go")

	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(projectRoot) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(globalSkill, "templates"), 0o755); err != nil {
		t.Fatalf("MkdirAll(globalSkill/templates) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(globalSkill, "SKILL.md"), []byte("---\nname: go\n---\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(SKILL.md) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(globalSkill, "templates", "snippet.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile(snippet.txt) error = %v", err)
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
	cmd := newCopyCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"go"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("first Execute() error = %v", err)
	}
	if got := buf.String(); !strings.Contains(got, "[OK] copied go\n") {
		t.Fatalf("first output missing copied line: %q", got)
	}

	if info, err := os.Lstat(projectSkill); err != nil {
		t.Fatalf("Lstat(project skill) error = %v", err)
	} else if info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("project skill should be a directory copy, found symlink")
	}
	if _, err := os.Stat(filepath.Join(projectSkill, "templates", "snippet.txt")); err != nil {
		t.Fatalf("Stat(copied nested file) error = %v", err)
	}

	buf.Reset()
	cmd = newCopyCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"go"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("second Execute() error = %v", err)
	}
	if got := buf.String(); !strings.Contains(got, "[WARN] skipped go (already exists)\n") {
		t.Fatalf("second output missing skipped warning: %q", got)
	}
}

func TestCopyCommandReturnsErrorWhenNoMatchingSkills(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(globalSkill, "SKILL.md"), []byte("x"), 0o644); err != nil {
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

	cmd := newCopyCmd()
	cmd.SetArgs([]string{"missing"})

	err = cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want no matching skills error")
	}
	if !strings.Contains(err.Error(), "no matching skills: missing") {
		t.Fatalf("Execute() error = %q, want no matching skills message", err)
	}
}
