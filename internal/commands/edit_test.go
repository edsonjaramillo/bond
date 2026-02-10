package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditCommandRequiresExactlyOneSkillArg(t *testing.T) {
	cmd := newEditCmd()
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want arg validation error")
	}
	if !strings.Contains(err.Error(), "accepts 1 arg(s), received 0") {
		t.Fatalf("Execute() error = %q, want exact args error", err)
	}

	cmd = newEditCmd()
	cmd.SetArgs([]string{"go", "rust"})

	err = cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want arg validation error")
	}
	if !strings.Contains(err.Error(), "accepts 1 arg(s), received 2") {
		t.Fatalf("Execute() error = %q, want exact args error", err)
	}
}

func TestEditCommandErrorsWhenEditorUnset(t *testing.T) {
	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "project")
	xdgConfig := filepath.Join(tmp, "xdg")
	storeSkill := filepath.Join(xdgConfig, "bond", "go")

	if err := os.MkdirAll(storeSkill, 0o755); err != nil {
		t.Fatalf("MkdirAll(storeSkill) error = %v", err)
	}
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(projectRoot) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(storeSkill, "SKILL.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(store SKILL.md) error = %v", err)
	}

	chdirForTest(t, projectRoot)

	t.Setenv("XDG_CONFIG_HOME", xdgConfig)
	t.Setenv("EDITOR", "")

	cmd := newEditCmd()
	cmd.SetArgs([]string{"go"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want unset editor error")
	}
	if !strings.Contains(err.Error(), "EDITOR is not set") {
		t.Fatalf("Execute() error = %q, want unset editor message", err)
	}
}

func TestEditCommandErrorsWhenNoMatchingSkill(t *testing.T) {
	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "project")
	xdgConfig := filepath.Join(tmp, "xdg")
	storeSkill := filepath.Join(xdgConfig, "bond", "go")

	if err := os.MkdirAll(storeSkill, 0o755); err != nil {
		t.Fatalf("MkdirAll(storeSkill) error = %v", err)
	}
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(projectRoot) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(storeSkill, "SKILL.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(store SKILL.md) error = %v", err)
	}

	chdirForTest(t, projectRoot)

	t.Setenv("XDG_CONFIG_HOME", xdgConfig)
	t.Setenv("EDITOR", "true")

	cmd := newEditCmd()
	cmd.SetArgs([]string{"missing"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want no matching skill error")
	}
	if !strings.Contains(err.Error(), "no matching skills: missing") {
		t.Fatalf("Execute() error = %q, want no matching skills message", err)
	}
}

func TestEditCommandOpensSkillMDWithEditor(t *testing.T) {
	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "project")
	xdgConfig := filepath.Join(tmp, "xdg")
	storeSkill := filepath.Join(xdgConfig, "bond", "go")
	recordedPath := filepath.Join(tmp, "editor-path.txt")
	editorScriptPath := filepath.Join(tmp, "fake-editor.sh")

	if err := os.MkdirAll(storeSkill, 0o755); err != nil {
		t.Fatalf("MkdirAll(storeSkill) error = %v", err)
	}
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(projectRoot) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(storeSkill, "SKILL.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(store SKILL.md) error = %v", err)
	}
	if err := os.WriteFile(editorScriptPath, []byte("#!/bin/sh\nprintf '%s' \"$1\" > \""+recordedPath+"\"\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(fake-editor.sh) error = %v", err)
	}

	chdirForTest(t, projectRoot)

	t.Setenv("XDG_CONFIG_HOME", xdgConfig)
	t.Setenv("EDITOR", editorScriptPath)

	buf := &bytes.Buffer{}
	cmd := newEditCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"go"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	gotPathBytes, err := os.ReadFile(recordedPath)
	if err != nil {
		t.Fatalf("ReadFile(recordedPath) error = %v", err)
	}
	gotPath := string(gotPathBytes)
	wantPath := filepath.Join(storeSkill, "SKILL.md")
	if gotPath != wantPath {
		t.Fatalf("recorded editor path = %q, want %q", gotPath, wantPath)
	}
}
