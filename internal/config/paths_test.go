package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestProjectDirsFrom verifies deterministic project directory derivation.
func TestProjectDirsFrom(t *testing.T) {
	root := "/tmp/work"
	if got := ProjectAgentsDirFrom(root); got != filepath.Join(root, ".agents") {
		t.Fatalf("ProjectAgentsDirFrom() = %q", got)
	}
	if got := ProjectSkillsDirFrom(root); got != filepath.Join(root, ".agents", "skills") {
		t.Fatalf("ProjectSkillsDirFrom() = %q", got)
	}
}

// TestGlobalSkillsDirPrefersXDG ensures XDG_CONFIG_HOME takes precedence.
func TestGlobalSkillsDirPrefersXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")
	got, err := GlobalSkillsDir()
	if err != nil {
		t.Fatalf("GlobalSkillsDir() error = %v", err)
	}
	want := filepath.Join("/tmp/xdg", "bond")
	if got != want {
		t.Fatalf("GlobalSkillsDir() = %q, want %q", got, want)
	}
}

// TestGlobalSkillsDirFallsBackToHome ensures home-based config is used without XDG.
func TestGlobalSkillsDirFallsBackToHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("os.UserHomeDir() error = %v", err)
	}

	got, err := GlobalSkillsDir()
	if err != nil {
		t.Fatalf("GlobalSkillsDir() error = %v", err)
	}
	want := filepath.Join(home, ".config", "bond")
	if got != want {
		t.Fatalf("GlobalSkillsDir() = %q, want %q", got, want)
	}
}
