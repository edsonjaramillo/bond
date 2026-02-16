package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHasSkillMarker(t *testing.T) {
	tmp := t.TempDir()

	missing, err := hasSkillMarker(tmp)
	if err != nil {
		t.Fatalf("hasSkillMarker(missing marker) error = %v", err)
	}
	if missing {
		t.Fatal("hasSkillMarker(missing marker) = true, want false")
	}

	if err := os.WriteFile(filepath.Join(tmp, "SKILL.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(SKILL.md) error = %v", err)
	}
	present, err := hasSkillMarker(tmp)
	if err != nil {
		t.Fatalf("hasSkillMarker(file marker) error = %v", err)
	}
	if !present {
		t.Fatal("hasSkillMarker(file marker) = false, want true")
	}

	tmpDirMarker := t.TempDir()
	if err := os.Mkdir(filepath.Join(tmpDirMarker, "SKILL.md"), 0o755); err != nil {
		t.Fatalf("Mkdir(SKILL.md directory) error = %v", err)
	}
	dirMarker, err := hasSkillMarker(tmpDirMarker)
	if err != nil {
		t.Fatalf("hasSkillMarker(directory marker) error = %v", err)
	}
	if dirMarker {
		t.Fatal("hasSkillMarker(directory marker) = true, want false")
	}
}
