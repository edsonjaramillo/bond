package skills

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDiscoverLinkedOnlyReturnsSymlinks filters out regular files.
func TestDiscoverLinkedOnlyReturnsSymlinks(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "regular")
	linkTarget := filepath.Join(tmp, "target")
	link := filepath.Join(tmp, "linked")

	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(file) error = %v", err)
	}
	if err := os.WriteFile(linkTarget, []byte("y"), 0o644); err != nil {
		t.Fatalf("WriteFile(target) error = %v", err)
	}
	if err := os.Symlink(linkTarget, link); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	entries, err := DiscoverLinked(tmp)
	if err != nil {
		t.Fatalf("DiscoverLinked() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("DiscoverLinked() len = %d", len(entries))
	}
	if entries[0].Name != "linked" {
		t.Fatalf("DiscoverLinked() name = %q", entries[0].Name)
	}
}

// TestUnlinkRemovesSymlinkAndSkipsNonSymlink verifies unlink safety checks.
func TestUnlinkRemovesSymlinkAndSkipsNonSymlink(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "target")
	link := filepath.Join(tmp, "linked")
	regular := filepath.Join(tmp, "regular")

	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(target) error = %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}
	if err := os.WriteFile(regular, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(regular) error = %v", err)
	}

	removed, err := Unlink(link)
	if err != nil {
		t.Fatalf("Unlink(link) error = %v", err)
	}
	if !removed {
		t.Fatalf("Unlink(link) removed = false")
	}

	removed, err = Unlink(regular)
	if err != nil {
		t.Fatalf("Unlink(regular) error = %v", err)
	}
	if removed {
		t.Fatalf("Unlink(regular) removed = true")
	}
}
