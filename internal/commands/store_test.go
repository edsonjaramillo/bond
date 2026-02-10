package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreCommandRequiresAtLeastOneSkillArg(t *testing.T) {
	cmd := newStoreCmd()
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want arg validation error")
	}
	if !strings.Contains(err.Error(), "requires at least 1 arg(s)") {
		t.Fatalf("Execute() error = %q, want minimum args error", err)
	}
}

func TestStoreCommandCopiesAndThenSkipsExisting(t *testing.T) {
	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "project")
	projectSkill := filepath.Join(projectRoot, ".agents", "skills", "go")
	xdgConfig := filepath.Join(tmp, "xdg")
	globalSkill := filepath.Join(xdgConfig, "bond", "go")

	if err := os.MkdirAll(filepath.Join(projectSkill, "templates"), 0o755); err != nil {
		t.Fatalf("MkdirAll(projectSkill/templates) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectSkill, "SKILL.md"), []byte("---\nname: go\n---\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(project SKILL.md) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectSkill, "templates", "snippet.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile(project snippet.txt) error = %v", err)
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
	cmd := newStoreCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"go"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("first Execute() error = %v", err)
	}
	if got := buf.String(); !strings.Contains(got, "[OK] stored go\n") {
		t.Fatalf("first output missing stored line: %q", got)
	}

	if info, err := os.Lstat(globalSkill); err != nil {
		t.Fatalf("Lstat(global skill) error = %v", err)
	} else if info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("global skill should be a directory copy, found symlink")
	}
	if _, err := os.Stat(filepath.Join(globalSkill, "templates", "snippet.txt")); err != nil {
		t.Fatalf("Stat(stored nested file) error = %v", err)
	}

	buf.Reset()
	cmd = newStoreCmd()
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

func TestStoreCommandReturnsErrorWhenNoMatchingSkills(t *testing.T) {
	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "project")
	projectSkill := filepath.Join(projectRoot, ".agents", "skills", "go")
	xdgConfig := filepath.Join(tmp, "xdg")

	if err := os.MkdirAll(projectSkill, 0o755); err != nil {
		t.Fatalf("MkdirAll(projectSkill) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectSkill, "SKILL.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(project SKILL.md) error = %v", err)
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

	cmd := newStoreCmd()
	cmd.SetArgs([]string{"missing"})

	err = cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want no matching skills error")
	}
	if !strings.Contains(err.Error(), "no matching skills: missing") {
		t.Fatalf("Execute() error = %q, want no matching skills message", err)
	}
}

func TestStoreCommandIgnoresSymlinkedProjectSkill(t *testing.T) {
	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "project")
	projectSkillsDir := filepath.Join(projectRoot, ".agents", "skills")
	xdgConfig := filepath.Join(tmp, "xdg")
	targetDir := filepath.Join(tmp, "target-go")

	if err := os.MkdirAll(projectSkillsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(projectSkillsDir) error = %v", err)
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(targetDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "SKILL.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(target SKILL.md) error = %v", err)
	}
	if err := os.Symlink(targetDir, filepath.Join(projectSkillsDir, "go")); err != nil {
		t.Fatalf("Symlink(go) error = %v", err)
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

	cmd := newStoreCmd()
	cmd.SetArgs([]string{"go"})

	err = cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want no matching skills error")
	}
	if !strings.Contains(err.Error(), "no matching skills: go") {
		t.Fatalf("Execute() error = %q, want no matching skills message", err)
	}
}

func TestCompleteProjectStorableSkillsListsOnlyValidNonSymlinked(t *testing.T) {
	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "project")
	projectSkillsDir := filepath.Join(projectRoot, ".agents", "skills")
	targetDir := filepath.Join(tmp, "target")

	if err := os.MkdirAll(filepath.Join(projectSkillsDir, "go"), 0o755); err != nil {
		t.Fatalf("MkdirAll(go) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(projectSkillsDir, "rust"), 0o755); err != nil {
		t.Fatalf("MkdirAll(rust) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(projectSkillsDir, "invalid"), 0o755); err != nil {
		t.Fatalf("MkdirAll(invalid) error = %v", err)
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(targetDir) error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(projectSkillsDir, "go", "SKILL.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(go/SKILL.md) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectSkillsDir, "rust", "SKILL.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(rust/SKILL.md) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectSkillsDir, "invalid", "README.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(invalid/README.md) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "SKILL.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(targetDir/SKILL.md) error = %v", err)
	}
	if err := os.Symlink(targetDir, filepath.Join(projectSkillsDir, "symlinked")); err != nil {
		t.Fatalf("Symlink(symlinked) error = %v", err)
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

	candidates, directive := completeProjectStorableSkills(newStoreCmd(), nil, "")
	if directive == 0 {
		t.Fatalf("directive = %d, want non-zero no-file-completion directive", directive)
	}
	if len(candidates) != 2 {
		t.Fatalf("len(candidates) = %d, want 2", len(candidates))
	}
	if candidates[0] != "go" {
		t.Fatalf("candidates[0] = %q, want go", candidates[0])
	}
	if candidates[1] != "rust" {
		t.Fatalf("candidates[1] = %q, want rust", candidates[1])
	}
}

func TestCompleteProjectStorableSkillsReturnsNoCandidatesOnDiscoverError(t *testing.T) {
	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "project")
	agentsDir := filepath.Join(projectRoot, ".agents")

	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(agentsDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, "skills"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(.agents/skills) error = %v", err)
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

	candidates, directive := completeProjectStorableSkills(newStoreCmd(), nil, "")
	if directive == 0 {
		t.Fatalf("directive = %d, want non-zero no-file-completion directive", directive)
	}
	if len(candidates) != 0 {
		t.Fatalf("len(candidates) = %d, want 0", len(candidates))
	}
}
