package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyRecursivelyCopiesDirectory(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "global", "go")
	dest := filepath.Join(tmp, "project", ".agents", "skills", "go")

	if err := os.MkdirAll(filepath.Join(source, "templates"), 0o755); err != nil {
		t.Fatalf("MkdirAll(source/templates) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte("go-skill"), 0o644); err != nil {
		t.Fatalf("WriteFile(SKILL.md) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "templates", "run.sh"), []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(run.sh) error = %v", err)
	}
	if err := os.Symlink("templates/run.sh", filepath.Join(source, "run-link")); err != nil {
		t.Fatalf("Symlink(run-link) error = %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatalf("MkdirAll(dest parent) error = %v", err)
	}

	result, err := Copy(source, dest)
	if err != nil {
		t.Fatalf("Copy() error = %v", err)
	}
	if result.Status != CopyStatusCopied {
		t.Fatalf("Copy() status = %q, want copied", result.Status)
	}

	if info, err := os.Lstat(dest); err != nil {
		t.Fatalf("Lstat(dest) error = %v", err)
	} else if !info.IsDir() {
		t.Fatalf("dest is not a directory")
	}

	got, err := os.ReadFile(filepath.Join(dest, "SKILL.md"))
	if err != nil {
		t.Fatalf("ReadFile(dest/SKILL.md) error = %v", err)
	}
	if string(got) != "go-skill" {
		t.Fatalf("SKILL.md contents = %q, want %q", string(got), "go-skill")
	}

	runPath := filepath.Join(dest, "templates", "run.sh")
	if info, err := os.Stat(runPath); err != nil {
		t.Fatalf("Stat(run.sh) error = %v", err)
	} else if info.Mode()&0o111 == 0 {
		t.Fatalf("run.sh is not executable, mode = %v", info.Mode())
	}

	target, err := os.Readlink(filepath.Join(dest, "run-link"))
	if err != nil {
		t.Fatalf("Readlink(run-link) error = %v", err)
	}
	if target != "templates/run.sh" {
		t.Fatalf("run-link target = %q, want %q", target, "templates/run.sh")
	}
}

func TestCopyReturnsConflictWhenDestinationExists(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "global", "go")
	dest := filepath.Join(tmp, "project", ".agents", "skills", "go")

	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("MkdirAll(source) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(source SKILL.md) error = %v", err)
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatalf("MkdirAll(dest) error = %v", err)
	}

	result, err := Copy(source, dest)
	if err != nil {
		t.Fatalf("Copy() error = %v", err)
	}
	if result.Status != CopyStatusConflict {
		t.Fatalf("Copy() status = %q, want conflict", result.Status)
	}
}

func TestCopyReturnsErrorWhenSourceMissing(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "global", "go")
	dest := filepath.Join(tmp, "project", ".agents", "skills", "go")

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatalf("MkdirAll(dest parent) error = %v", err)
	}

	_, err := Copy(source, dest)
	if err == nil {
		t.Fatal("Copy() error = nil, want non-nil")
	}
}

func TestCopyReturnsErrorWhenSourceIsNotDirectory(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "global", "go")
	dest := filepath.Join(tmp, "project", ".agents", "skills", "go")

	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatalf("MkdirAll(source parent) error = %v", err)
	}
	if err := os.WriteFile(source, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(source) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatalf("MkdirAll(dest parent) error = %v", err)
	}

	_, err := Copy(source, dest)
	if err == nil {
		t.Fatal("Copy() error = nil, want non-nil")
	}
}
