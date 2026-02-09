package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListCommandDefaultListsGlobalSkills(t *testing.T) {
	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "project")
	xdgConfig := filepath.Join(tmp, "xdg")
	globalSkills := filepath.Join(xdgConfig, "bond")

	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(projectRoot) error = %v", err)
	}
	if err := os.MkdirAll(globalSkills, 0o755); err != nil {
		t.Fatalf("MkdirAll(globalSkills) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(globalSkills, "go"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(go) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(globalSkills, "rust"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(rust) error = %v", err)
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
	cmd := newListCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "go\n") {
		t.Fatalf("output missing go skill: %q", output)
	}
	if !strings.Contains(output, "rust\n") {
		t.Fatalf("output missing rust skill: %q", output)
	}
	if !strings.Contains(output, "summary total=2") {
		t.Fatalf("output missing summary: %q", output)
	}
}

func TestListCommandProjectFlagListsOnlyGlobalLinks(t *testing.T) {
	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "project")
	projectSkills := filepath.Join(projectRoot, ".agents", "skills")
	xdgConfig := filepath.Join(tmp, "xdg")
	globalSkills := filepath.Join(xdgConfig, "bond")
	external := filepath.Join(tmp, "external")

	if err := os.MkdirAll(projectSkills, 0o755); err != nil {
		t.Fatalf("MkdirAll(projectSkills) error = %v", err)
	}
	if err := os.MkdirAll(globalSkills, 0o755); err != nil {
		t.Fatalf("MkdirAll(globalSkills) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(globalSkills, "go"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(go) error = %v", err)
	}
	if err := os.WriteFile(external, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(external) error = %v", err)
	}

	if err := os.Symlink(filepath.Join(globalSkills, "go"), filepath.Join(projectSkills, "go")); err != nil {
		t.Fatalf("Symlink(go) error = %v", err)
	}
	if err := os.Symlink(external, filepath.Join(projectSkills, "external")); err != nil {
		t.Fatalf("Symlink(external) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectSkills, "regular"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(regular) error = %v", err)
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
	cmd := newListCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--project"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "go\n") {
		t.Fatalf("output missing go skill: %q", output)
	}
	if strings.Contains(output, "external\n") {
		t.Fatalf("output unexpectedly includes external skill: %q", output)
	}
	if strings.Contains(output, "regular\n") {
		t.Fatalf("output unexpectedly includes regular entry: %q", output)
	}
	if !strings.Contains(output, "summary linked=1") {
		t.Fatalf("output missing summary: %q", output)
	}
}
