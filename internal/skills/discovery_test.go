package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverFindsNestedSkillDirsSorted(t *testing.T) {
	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, "global")

	mustMkdirAll(t, filepath.Join(sourceDir, "lang", "go"))
	mustMkdirAll(t, filepath.Join(sourceDir, "frontend", "react"))
	mustMkdirAll(t, filepath.Join(sourceDir, "invalid", "python"))
	mustMkdirAll(t, filepath.Join(sourceDir, "ops", "terraform"))

	mustWriteFile(t, filepath.Join(sourceDir, "lang", "go", "SKILL.md"), "go")
	mustWriteFile(t, filepath.Join(sourceDir, "frontend", "react", "SKILL.md"), "react")
	mustWriteFile(t, filepath.Join(sourceDir, "invalid", "python", "skill.md"), "wrong-case")
	mustWriteFile(t, filepath.Join(sourceDir, "ops", "terraform", "SKILL.MD"), "wrong-case")
	mustWriteFile(t, filepath.Join(sourceDir, "SKILL.md"), "root marker should be ignored")

	skills, err := Discover(sourceDir)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(skills) != 2 {
		t.Fatalf("len(skills) = %d, want 2", len(skills))
	}
	if skills[0].Name != "go" {
		t.Fatalf("skills[0].Name = %q, want go", skills[0].Name)
	}
	if skills[0].Path != filepath.Join(sourceDir, "lang", "go") {
		t.Fatalf("skills[0].Path = %q", skills[0].Path)
	}
	if skills[1].Name != "react" {
		t.Fatalf("skills[1].Name = %q, want react", skills[1].Name)
	}
	if skills[1].Path != filepath.Join(sourceDir, "frontend", "react") {
		t.Fatalf("skills[1].Path = %q", skills[1].Path)
	}
}

func TestDiscoverMissingDirReturnsEmpty(t *testing.T) {
	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, "does-not-exist")

	skills, err := Discover(sourceDir)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(skills) != 0 {
		t.Fatalf("len(skills) = %d, want 0", len(skills))
	}
}

func TestDiscoverDuplicateSkillNamesReturnsError(t *testing.T) {
	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, "global")

	mustMkdirAll(t, filepath.Join(sourceDir, "team-a", "go"))
	mustMkdirAll(t, filepath.Join(sourceDir, "team-b", "go"))
	mustWriteFile(t, filepath.Join(sourceDir, "team-a", "go", "SKILL.md"), "go")
	mustWriteFile(t, filepath.Join(sourceDir, "team-b", "go", "SKILL.md"), "go")

	_, err := Discover(sourceDir)
	if err == nil {
		t.Fatal("Discover() error = nil, want duplicate-skill error")
	}
	if !strings.Contains(err.Error(), "duplicate skill") {
		t.Fatalf("Discover() error = %q, want duplicate skill message", err)
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
