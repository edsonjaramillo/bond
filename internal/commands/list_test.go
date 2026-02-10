package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListCommandDefaultListsProjectSkills(t *testing.T) {
	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "project")
	projectSkills := filepath.Join(projectRoot, ".agents", "skills")
	xdgConfig := filepath.Join(tmp, "xdg")
	storeSkills := filepath.Join(xdgConfig, "bond")
	externalDir := filepath.Join(tmp, "external-skill")
	missingMarkerDir := filepath.Join(tmp, "missing-marker")
	fileTarget := filepath.Join(tmp, "file-target")

	if err := os.MkdirAll(projectSkills, 0o755); err != nil {
		t.Fatalf("MkdirAll(projectSkills) error = %v", err)
	}
	if err := os.MkdirAll(storeSkills, 0o755); err != nil {
		t.Fatalf("MkdirAll(storeSkills) error = %v", err)
	}

	if err := os.MkdirAll(filepath.Join(projectSkills, "go"), 0o755); err != nil {
		t.Fatalf("MkdirAll(go) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectSkills, "go", "SKILL.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(go/SKILL.md) error = %v", err)
	}

	if err := os.MkdirAll(externalDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(externalDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(externalDir, "SKILL.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(externalDir/SKILL.md) error = %v", err)
	}
	if err := os.Symlink(externalDir, filepath.Join(projectSkills, "linked-ext")); err != nil {
		t.Fatalf("Symlink(linked-ext) error = %v", err)
	}

	if err := os.MkdirAll(missingMarkerDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(missingMarkerDir) error = %v", err)
	}
	if err := os.Symlink(missingMarkerDir, filepath.Join(projectSkills, "invalid-symlink")); err != nil {
		t.Fatalf("Symlink(invalid-symlink) error = %v", err)
	}

	if err := os.WriteFile(fileTarget, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(fileTarget) error = %v", err)
	}
	if err := os.Symlink(fileTarget, filepath.Join(projectSkills, "file-link")); err != nil {
		t.Fatalf("Symlink(file-link) error = %v", err)
	}

	if err := os.MkdirAll(filepath.Join(projectSkills, "invalid-local"), 0o755); err != nil {
		t.Fatalf("MkdirAll(invalid-local) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectSkills, "regular-file"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(regular-file) error = %v", err)
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
	if !strings.Contains(output, "[INFO] go\n") {
		t.Fatalf("output missing go skill: %q", output)
	}
	if !strings.Contains(output, "[INFO] linked-ext\n") {
		t.Fatalf("output missing linked-ext skill: %q", output)
	}
	if strings.Contains(output, "[INFO] invalid-symlink\n") {
		t.Fatalf("output unexpectedly includes invalid symlink: %q", output)
	}
	if strings.Contains(output, "[INFO] file-link\n") {
		t.Fatalf("output unexpectedly includes symlink to file: %q", output)
	}
	if strings.Contains(output, "[INFO] invalid-local\n") {
		t.Fatalf("output unexpectedly includes invalid local entry: %q", output)
	}
	if strings.Contains(output, "[INFO] regular-file\n") {
		t.Fatalf("output unexpectedly includes regular file: %q", output)
	}
}

func TestListCommandStoreFlagListsStoreSkills(t *testing.T) {
	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "project")
	projectSkills := filepath.Join(projectRoot, ".agents", "skills")
	xdgConfig := filepath.Join(tmp, "xdg")
	storeSkills := filepath.Join(xdgConfig, "bond")

	if err := os.MkdirAll(projectSkills, 0o755); err != nil {
		t.Fatalf("MkdirAll(projectSkills) error = %v", err)
	}
	if err := os.MkdirAll(storeSkills, 0o755); err != nil {
		t.Fatalf("MkdirAll(storeSkills) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(projectSkills, "go"), 0o755); err != nil {
		t.Fatalf("MkdirAll(project go) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectSkills, "go", "SKILL.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(go/SKILL.md) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(storeSkills, "lang", "rust"), 0o755); err != nil {
		t.Fatalf("MkdirAll(store rust) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(storeSkills, "lang", "rust", "SKILL.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(store rust/SKILL.md) error = %v", err)
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
	cmd.SetArgs([]string{"--store"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "[INFO] rust\n") {
		t.Fatalf("output missing store rust skill: %q", output)
	}
	if strings.Contains(output, "[INFO] go\n") {
		t.Fatalf("output unexpectedly includes project skill go: %q", output)
	}
}

func TestListCommandProjectFlagIsUnknown(t *testing.T) {
	buf := &bytes.Buffer{}
	cmd := newListCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--project"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want unknown flag error")
	}
	if !strings.Contains(err.Error(), "unknown flag: --project") {
		t.Fatalf("error = %q, want unknown --project flag", err)
	}
}
