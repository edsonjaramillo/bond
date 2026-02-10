package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverProjectStorableFiltersAndSorts(t *testing.T) {
	tmp := t.TempDir()
	projectSkillsDir := filepath.Join(tmp, "project", ".agents", "skills")

	if err := os.MkdirAll(projectSkillsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(projectSkillsDir) error = %v", err)
	}

	goDir := filepath.Join(projectSkillsDir, "go")
	rustDir := filepath.Join(projectSkillsDir, "rust")
	invalidDir := filepath.Join(projectSkillsDir, "invalid")
	targetDir := filepath.Join(tmp, "symlink-target")

	if err := os.MkdirAll(goDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(goDir) error = %v", err)
	}
	if err := os.MkdirAll(rustDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(rustDir) error = %v", err)
	}
	if err := os.MkdirAll(invalidDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(invalidDir) error = %v", err)
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(targetDir) error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(goDir, "SKILL.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(go/SKILL.md) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(rustDir, "SKILL.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(rust/SKILL.md) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(invalidDir, "README.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(invalid/README.md) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectSkillsDir, "not-a-dir"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(not-a-dir) error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(targetDir, "SKILL.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(targetDir/SKILL.md) error = %v", err)
	}
	if err := os.Symlink(targetDir, filepath.Join(projectSkillsDir, "symlinked")); err != nil {
		t.Fatalf("Symlink(symlinked) error = %v", err)
	}

	discovered, err := DiscoverProjectStorable(projectSkillsDir)
	if err != nil {
		t.Fatalf("DiscoverProjectStorable() error = %v", err)
	}

	if len(discovered) != 2 {
		t.Fatalf("len(discovered) = %d, want 2", len(discovered))
	}
	if discovered[0].Name != "go" {
		t.Fatalf("discovered[0].Name = %q, want go", discovered[0].Name)
	}
	if discovered[1].Name != "rust" {
		t.Fatalf("discovered[1].Name = %q, want rust", discovered[1].Name)
	}
}

func TestDiscoverProjectStorableMissingDirReturnsEmpty(t *testing.T) {
	tmp := t.TempDir()
	projectSkillsDir := filepath.Join(tmp, "project", ".agents", "skills")

	discovered, err := DiscoverProjectStorable(projectSkillsDir)
	if err != nil {
		t.Fatalf("DiscoverProjectStorable() error = %v", err)
	}
	if len(discovered) != 0 {
		t.Fatalf("len(discovered) = %d, want 0", len(discovered))
	}
}

func TestDiscoverProjectAllIncludesLocalAndSymlinkSkills(t *testing.T) {
	tmp := t.TempDir()
	projectSkillsDir := filepath.Join(tmp, "project", ".agents", "skills")
	externalDir := filepath.Join(tmp, "external-skill")
	missingMarkerDir := filepath.Join(tmp, "missing-marker")
	fileTarget := filepath.Join(tmp, "file-target")

	if err := os.MkdirAll(projectSkillsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(projectSkillsDir) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(projectSkillsDir, "go"), 0o755); err != nil {
		t.Fatalf("MkdirAll(go) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectSkillsDir, "go", "SKILL.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(go/SKILL.md) error = %v", err)
	}

	if err := os.MkdirAll(externalDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(externalDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(externalDir, "SKILL.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(externalDir/SKILL.md) error = %v", err)
	}
	if err := os.Symlink(externalDir, filepath.Join(projectSkillsDir, "linked-ext")); err != nil {
		t.Fatalf("Symlink(linked-ext) error = %v", err)
	}

	if err := os.MkdirAll(missingMarkerDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(missingMarkerDir) error = %v", err)
	}
	if err := os.Symlink(missingMarkerDir, filepath.Join(projectSkillsDir, "missing-skill")); err != nil {
		t.Fatalf("Symlink(missing-skill) error = %v", err)
	}

	if err := os.WriteFile(fileTarget, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(fileTarget) error = %v", err)
	}
	if err := os.Symlink(fileTarget, filepath.Join(projectSkillsDir, "file-link")); err != nil {
		t.Fatalf("Symlink(file-link) error = %v", err)
	}

	if err := os.MkdirAll(filepath.Join(projectSkillsDir, "invalid"), 0o755); err != nil {
		t.Fatalf("MkdirAll(invalid) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectSkillsDir, "regular"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(regular) error = %v", err)
	}

	discovered, err := DiscoverProjectAll(projectSkillsDir)
	if err != nil {
		t.Fatalf("DiscoverProjectAll() error = %v", err)
	}

	if len(discovered) != 2 {
		t.Fatalf("len(discovered) = %d, want 2", len(discovered))
	}
	if discovered[0].Name != "go" {
		t.Fatalf("discovered[0].Name = %q, want go", discovered[0].Name)
	}
	if discovered[1].Name != "linked-ext" {
		t.Fatalf("discovered[1].Name = %q, want linked-ext", discovered[1].Name)
	}
}

func TestDiscoverProjectAllMissingDirReturnsEmpty(t *testing.T) {
	tmp := t.TempDir()
	projectSkillsDir := filepath.Join(tmp, "project", ".agents", "skills")

	discovered, err := DiscoverProjectAll(projectSkillsDir)
	if err != nil {
		t.Fatalf("DiscoverProjectAll() error = %v", err)
	}
	if len(discovered) != 0 {
		t.Fatalf("len(discovered) = %d, want 0", len(discovered))
	}
}
