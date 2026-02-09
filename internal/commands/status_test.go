package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStatusCommandPrintsEntries(t *testing.T) {
	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "project")
	projectSkills := filepath.Join(projectRoot, ".agents", "skills")
	xdgConfig := filepath.Join(tmp, "xdg")
	globalSkills := filepath.Join(xdgConfig, "bond")

	if err := os.MkdirAll(projectSkills, 0o755); err != nil {
		t.Fatalf("MkdirAll(projectSkills) error = %v", err)
	}
	if err := os.MkdirAll(globalSkills, 0o755); err != nil {
		t.Fatalf("MkdirAll(globalSkills) error = %v", err)
	}

	target := filepath.Join(globalSkills, "go")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(target) error = %v", err)
	}
	if err := os.Symlink(target, filepath.Join(projectSkills, "go")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
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
	cmd := newStatusCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "[OK] linked go") {
		t.Fatalf("output missing linked entry: %q", output)
	}
}
