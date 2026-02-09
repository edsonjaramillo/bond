package skills

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLinkCreatesSymlinkAndIsIdempotent verifies create-then-retry behavior.
func TestLinkCreatesSymlinkAndIsIdempotent(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "source-skill")
	dest := filepath.Join(tmp, "dest-skill")

	if err := os.WriteFile(source, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, err := Link(source, dest)
	if err != nil {
		t.Fatalf("Link() error = %v", err)
	}
	if result.Status != LinkStatusLinked {
		t.Fatalf("Link() status = %q", result.Status)
	}

	result, err = Link(source, dest)
	if err != nil {
		t.Fatalf("Link() second error = %v", err)
	}
	if result.Status != LinkStatusAlreadyLinked {
		t.Fatalf("Link() second status = %q", result.Status)
	}
}

// TestLinkDetectsConflict verifies conflicting destinations are reported.
func TestLinkDetectsConflict(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "source-skill")
	dest := filepath.Join(tmp, "dest-skill")
	other := filepath.Join(tmp, "other")

	if err := os.WriteFile(source, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(source) error = %v", err)
	}
	if err := os.WriteFile(other, []byte("y"), 0o644); err != nil {
		t.Fatalf("WriteFile(other) error = %v", err)
	}
	if err := os.Symlink(other, dest); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	result, err := Link(source, dest)
	if err != nil {
		t.Fatalf("Link() error = %v", err)
	}
	if result.Status != LinkStatusConflict {
		t.Fatalf("Link() status = %q", result.Status)
	}
}
