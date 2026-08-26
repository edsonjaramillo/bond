package main

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestExecutableWiring(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "bond")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}

	const injectedVersion = "v1.2.3-test"
	build := exec.Command(
		"go", "build",
		"-ldflags", "-X github.com/edsonjaramillo/bond/internal/application.Version="+injectedVersion,
		"-o", binary,
		".",
	)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build executable: %v\n%s", err, output)
	}

	command := exec.Command(binary, "--version")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("run executable: %v\n%s", err, output)
	}
	if got, want := string(output), injectedVersion+"\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}
