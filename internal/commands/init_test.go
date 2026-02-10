package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func chdirForTest(t *testing.T, dir string) {
	t.Helper()

	prevWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}
	t.Cleanup(func() {
		if chdirErr := os.Chdir(prevWd); chdirErr != nil {
			t.Fatalf("restore cwd error = %v", chdirErr)
		}
	})
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("os.Chdir() error = %v", err)
	}
}

func TestInitCommandWhenDirectoriesAlreadyExist(t *testing.T) {
	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "project")
	projectSkills := filepath.Join(projectRoot, ".agents", "skills")
	if err := os.MkdirAll(projectSkills, 0o755); err != nil {
		t.Fatalf("MkdirAll(projectSkills) error = %v", err)
	}

	chdirForTest(t, projectRoot)

	buf := &bytes.Buffer{}
	cmd := newInitCmd()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if got, want := buf.String(), "[INFO] .agents/skills already exists\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestInitCommandWhenDirectoriesAreMissing(t *testing.T) {
	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "project")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(projectRoot) error = %v", err)
	}

	chdirForTest(t, projectRoot)

	buf := &bytes.Buffer{}
	cmd := newInitCmd()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if got, want := buf.String(), "[OK] initialized .agents/skills\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}

	if _, err := os.Stat(filepath.Join(projectRoot, ".agents")); err != nil {
		t.Fatalf("Stat(.agents) error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, ".agents", "skills")); err != nil {
		t.Fatalf("Stat(.agents/skills) error = %v", err)
	}
}

func TestInitCommandWhenDirectoriesAreMixed(t *testing.T) {
	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "project")
	if err := os.MkdirAll(filepath.Join(projectRoot, ".agents"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.agents) error = %v", err)
	}

	chdirForTest(t, projectRoot)

	buf := &bytes.Buffer{}
	cmd := newInitCmd()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if got, want := buf.String(), "[OK] initialized .agents/skills\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestInitCommandReturnsErrorWhenAgentsPathIsFile(t *testing.T) {
	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "project")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(projectRoot) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".agents"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(.agents) error = %v", err)
	}

	chdirForTest(t, projectRoot)

	buf := &bytes.Buffer{}
	cmd := newInitCmd()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), `exists and is not a directory`) {
		t.Fatalf("error = %q, want non-directory message", err.Error())
	}
	if got := buf.String(); got != "" {
		t.Fatalf("output = %q, want empty output", got)
	}
}

func TestInitCommandGlobalWhenDirectoryMissing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	globalDir := filepath.Join(tmp, "bond")

	buf := &bytes.Buffer{}
	cmd := newInitCmd()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--store"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if got, want := buf.String(), "[OK] initialized store bond directory\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
	if _, err := os.Stat(globalDir); err != nil {
		t.Fatalf("Stat(globalDir) error = %v", err)
	}
}

func TestInitCommandGlobalWhenDirectoryExists(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	globalDir := filepath.Join(tmp, "bond")
	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(globalDir) error = %v", err)
	}

	buf := &bytes.Buffer{}
	cmd := newInitCmd()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--store"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if got, want := buf.String(), "[INFO] store bond directory already exists\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestInitCommandGlobalReturnsErrorWhenPathIsFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	globalPath := filepath.Join(tmp, "bond")
	if err := os.WriteFile(globalPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(globalPath) error = %v", err)
	}

	buf := &bytes.Buffer{}
	cmd := newInitCmd()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--store"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), `exists and is not a directory`) {
		t.Fatalf("error = %q, want non-directory message", err.Error())
	}
	if got := buf.String(); got != "" {
		t.Fatalf("output = %q, want empty output", got)
	}
}
