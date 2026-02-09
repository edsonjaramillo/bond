package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverProjectLinkedGlobalFiltersAndSorts(t *testing.T) {
	tmp := t.TempDir()
	globalDir := filepath.Join(tmp, "global")
	projectDir := filepath.Join(tmp, "project", ".agents", "skills")
	externalDir := filepath.Join(tmp, "external")

	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(globalDir) error = %v", err)
	}
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(projectDir) error = %v", err)
	}
	if err := os.MkdirAll(externalDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(externalDir) error = %v", err)
	}

	globalA := filepath.Join(globalDir, "a-skill")
	globalZ := filepath.Join(globalDir, "z-skill")
	external := filepath.Join(externalDir, "other")

	if err := os.WriteFile(globalA, []byte("a"), 0o644); err != nil {
		t.Fatalf("WriteFile(globalA) error = %v", err)
	}
	if err := os.WriteFile(globalZ, []byte("z"), 0o644); err != nil {
		t.Fatalf("WriteFile(globalZ) error = %v", err)
	}
	if err := os.WriteFile(external, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(external) error = %v", err)
	}

	if err := os.Symlink(globalZ, filepath.Join(projectDir, "z-skill")); err != nil {
		t.Fatalf("Symlink(globalZ) error = %v", err)
	}
	if err := os.Symlink(globalA, filepath.Join(projectDir, "a-skill")); err != nil {
		t.Fatalf("Symlink(globalA) error = %v", err)
	}
	if err := os.Symlink(external, filepath.Join(projectDir, "external")); err != nil {
		t.Fatalf("Symlink(external) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "regular"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(regular) error = %v", err)
	}

	entries, err := DiscoverProjectLinkedGlobal(projectDir, globalDir)
	if err != nil {
		t.Fatalf("DiscoverProjectLinkedGlobal() error = %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	if entries[0].Name != "a-skill" {
		t.Fatalf("entries[0].Name = %q, want a-skill", entries[0].Name)
	}
	if entries[1].Name != "z-skill" {
		t.Fatalf("entries[1].Name = %q, want z-skill", entries[1].Name)
	}
}

func TestDiscoverProjectLinkedGlobalMissingProjectDir(t *testing.T) {
	tmp := t.TempDir()
	globalDir := filepath.Join(tmp, "global")
	projectDir := filepath.Join(tmp, "project", ".agents", "skills")

	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(globalDir) error = %v", err)
	}

	entries, err := DiscoverProjectLinkedGlobal(projectDir, globalDir)
	if err != nil {
		t.Fatalf("DiscoverProjectLinkedGlobal() error = %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("len(entries) = %d, want 0", len(entries))
	}
}
