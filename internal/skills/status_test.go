package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInspectStatusClassifiesEntries(t *testing.T) {
	tmp := t.TempDir()
	globalDir := filepath.Join(tmp, "global")
	projectDir := filepath.Join(tmp, "project", ".agents", "skills")
	externalTarget := filepath.Join(tmp, "external-skill")

	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(globalDir) error = %v", err)
	}
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(projectDir) error = %v", err)
	}

	linkedTarget := filepath.Join(globalDir, "linked-skill")
	if err := os.WriteFile(linkedTarget, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(linkedTarget) error = %v", err)
	}
	if err := os.WriteFile(externalTarget, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(externalTarget) error = %v", err)
	}

	if err := os.Symlink(linkedTarget, filepath.Join(projectDir, "linked-skill")); err != nil {
		t.Fatalf("Symlink(linked) error = %v", err)
	}
	if err := os.Symlink(filepath.Join(globalDir, "missing-skill"), filepath.Join(projectDir, "broken-skill")); err != nil {
		t.Fatalf("Symlink(broken) error = %v", err)
	}
	if err := os.Symlink(externalTarget, filepath.Join(projectDir, "external-skill")); err != nil {
		t.Fatalf("Symlink(external) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "conflict-skill"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(conflict) error = %v", err)
	}

	report, err := InspectStatus(globalDir, projectDir)
	if err != nil {
		t.Fatalf("InspectStatus() error = %v", err)
	}

	got := map[string]StatusKind{}
	for _, entry := range report.Entries {
		got[entry.Name] = entry.Status
	}

	if got["linked-skill"] != StatusLinked {
		t.Fatalf("linked-skill status = %q", got["linked-skill"])
	}
	if got["broken-skill"] != StatusBroken {
		t.Fatalf("broken-skill status = %q", got["broken-skill"])
	}
	if got["external-skill"] != StatusExternal {
		t.Fatalf("external-skill status = %q", got["external-skill"])
	}
	if got["conflict-skill"] != StatusConflict {
		t.Fatalf("conflict-skill status = %q", got["conflict-skill"])
	}
}

func TestInspectStatusMissingProjectDir(t *testing.T) {
	tmp := t.TempDir()
	globalDir := filepath.Join(tmp, "global")
	projectDir := filepath.Join(tmp, "project", ".agents", "skills")

	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(globalDir) error = %v", err)
	}

	report, err := InspectStatus(globalDir, projectDir)
	if err != nil {
		t.Fatalf("InspectStatus() error = %v", err)
	}

	if len(report.Entries) != 0 {
		t.Fatalf("InspectStatus() entries len = %d", len(report.Entries))
	}
}
