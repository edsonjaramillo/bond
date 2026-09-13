package application

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestInstructionsAppendCreatesDefaultTargetFromTopLevelInstruction(t *testing.T) {
	project := t.TempDir()
	configDirectory := t.TempDir()
	instructionStore := filepath.Join(configDirectory, "bond", "instructions")
	if err := os.MkdirAll(instructionStore, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(instructionStore, "format-lint.md"), []byte("Run the formatter, then the linter."), 0o644); err != nil {
		t.Fatal(err)
	}

	got := runApplicationInDirectory(t, project, []string{"XDG_CONFIG_HOME=" + configDirectory}, "", "instructions", "append", "format-lint.md")
	if got.exitCode != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", got.exitCode, got.stderr)
	}
	if got.stdout != "" || got.stderr != "" {
		t.Errorf("output = stdout %q, stderr %q; want silent success", got.stdout, got.stderr)
	}
	contents, err := os.ReadFile(filepath.Join(project, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "Run the formatter, then the linter.\n" {
		t.Errorf("AGENTS.md = %q, want appended Instruction ending in LF", contents)
	}
	info, err := os.Stat(filepath.Join(project, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if gotMode := info.Mode().Perm(); gotMode != 0o644 {
		t.Errorf("AGENTS.md mode = %04o, want 0644", gotMode)
	}
}

func TestBareInstructionsPrintsHelpWithOnlyAppend(t *testing.T) {
	t.Parallel()

	got := runApplication(t, "instructions")
	if got.exitCode != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", got.exitCode, got.stderr)
	}
	if !strings.Contains(got.stdout, "Usage:\n  bond instructions") || !strings.Contains(got.stdout, "append") {
		t.Errorf("stdout = %q, want Instructions help containing append", got.stdout)
	}
	if strings.Contains(got.stdout, "remove") || strings.Contains(got.stdout, "list") {
		t.Errorf("stdout = %q, want append as the only Instructions subcommand", got.stdout)
	}
	if got.stderr != "" {
		t.Errorf("stderr = %q, want empty", got.stderr)
	}
}

func TestInstructionsAppendRequiresExactlyOneInstructionPath(t *testing.T) {
	t.Parallel()

	for _, arguments := range [][]string{
		{"instructions", "append"},
		{"instructions", "append", "one.md", "two.md"},
	} {
		got := runApplication(t, arguments...)
		if got.exitCode != 1 {
			t.Errorf("Run(%q) exit code = %d, want 1", arguments, got.exitCode)
		}
		if got.stdout != "" || got.stderr == "" || strings.Contains(got.stderr, "Usage:") {
			t.Errorf("Run(%q) output = stdout %q, stderr %q; want one plain diagnostic", arguments, got.stdout, got.stderr)
		}
	}
}

func TestInstructionsAppendRejectsInvalidOrMissingSourcesWithoutCreatingFiles(t *testing.T) {
	t.Parallel()

	for _, instructionPath := range []string{"", "Format.md", "format_lint.md", "format-lint", "../format-lint.md", "group/format-lint.md", "format--lint.md"} {
		if instructionPath == "" {
			continue // ExactArgs covers the empty positional case.
		}
		project := t.TempDir()
		configDirectory := t.TempDir()
		got := runApplicationInDirectory(t, project, []string{"XDG_CONFIG_HOME=" + configDirectory}, "", "instructions", "append", instructionPath)
		if got.exitCode != 1 || got.stderr == "" {
			t.Errorf("append %q = exit %d, stderr %q; want failure", instructionPath, got.exitCode, got.stderr)
		}
		if _, err := os.Lstat(filepath.Join(project, "AGENTS.md")); !os.IsNotExist(err) {
			t.Errorf("append %q created target; Lstat error = %v", instructionPath, err)
		}
		if _, err := os.Lstat(filepath.Join(configDirectory, "bond")); !os.IsNotExist(err) {
			t.Errorf("append %q created Store infrastructure; Lstat error = %v", instructionPath, err)
		}
	}

	project := t.TempDir()
	configDirectory := t.TempDir()
	got := runApplicationInDirectory(t, project, []string{"XDG_CONFIG_HOME=" + configDirectory}, "", "instructions", "append", "missing.md")
	if got.exitCode != 1 || !strings.Contains(got.stderr, "missing.md") {
		t.Errorf("missing Instruction = exit %d, stderr %q; want named failure", got.exitCode, got.stderr)
	}
	if _, err := os.Lstat(filepath.Join(project, "AGENTS.md")); !os.IsNotExist(err) {
		t.Errorf("missing Instruction created target; Lstat error = %v", err)
	}
}

func TestInstructionsAppendValidatesInstructionContentAndIdentity(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		contents []byte
		setup    func(string) error
	}{
		{name: "empty", contents: nil},
		{name: "Unicode whitespace", contents: []byte(" \t\n\u2003")},
		{name: "invalid UTF-8", contents: []byte{0xff}},
		{name: "leading BOM", contents: append([]byte{0xef, 0xbb, 0xbf}, []byte("text")...)},
		{name: "directory", setup: func(path string) error { return os.Mkdir(path, 0o755) }},
		{name: "symlink", setup: func(path string) error {
			target := filepath.Join(filepath.Dir(path), "real.md")
			if err := os.WriteFile(target, []byte("text"), 0o644); err != nil {
				return err
			}
			return os.Symlink(target, path)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			project := t.TempDir()
			configDirectory := t.TempDir()
			store := filepath.Join(configDirectory, "bond", "instructions")
			if err := os.MkdirAll(store, 0o755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(store, "check.md")
			var err error
			if test.setup != nil {
				err = test.setup(path)
			} else {
				err = os.WriteFile(path, test.contents, 0o644)
			}
			if err != nil {
				t.Fatal(err)
			}

			got := runApplicationInDirectory(t, project, []string{"XDG_CONFIG_HOME=" + configDirectory}, "", "instructions", "append", "check.md")
			if got.exitCode != 1 || got.stderr == "" {
				t.Errorf("exit code = %d, stderr = %q; want validation failure", got.exitCode, got.stderr)
			}
			if _, err := os.Lstat(filepath.Join(project, "AGENTS.md")); !os.IsNotExist(err) {
				t.Errorf("invalid Instruction created target; Lstat error = %v", err)
			}
		})
	}
}

func TestInstructionsAppendPreservesBytesBoundariesDuplicatesAndTargetIdentity(t *testing.T) {
	for _, test := range []struct {
		name        string
		target      string
		instruction string
		want        string
	}{
		{name: "empty target", instruction: "instruction", want: "instruction\n"},
		{name: "no boundary LF", target: "target", instruction: "instruction", want: "target\n\ninstruction\n"},
		{name: "one boundary LF", target: "target\n", instruction: "instruction\n", want: "target\n\ninstruction\n"},
		{name: "existing blank line", target: "target\n\n\n", instruction: "instruction\n\n", want: "target\n\n\ninstruction\n\n"},
		{name: "leading Instruction LF contributes", target: "target\n", instruction: "\ninstruction", want: "target\n\ninstruction\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			project := t.TempDir()
			configDirectory := t.TempDir()
			writeInstructionForTest(t, configDirectory, "check.md", []byte(test.instruction))
			targetPath := filepath.Join(project, "AGENTS.md")
			if err := os.WriteFile(targetPath, []byte(test.target), 0o600); err != nil {
				t.Fatal(err)
			}
			before, err := os.Stat(targetPath)
			if err != nil {
				t.Fatal(err)
			}
			beforeStat := before.Sys().(*syscall.Stat_t)

			got := runApplicationInDirectory(t, project, []string{"XDG_CONFIG_HOME=" + configDirectory}, "", "instructions", "append", "check.md")
			if got.exitCode != 0 || got.stdout != "" || got.stderr != "" {
				t.Fatalf("result = exit %d, stdout %q, stderr %q; want silent success", got.exitCode, got.stdout, got.stderr)
			}
			contents, err := os.ReadFile(targetPath)
			if err != nil {
				t.Fatal(err)
			}
			if string(contents) != test.want {
				t.Errorf("target = %q, want %q", contents, test.want)
			}
			after, err := os.Stat(targetPath)
			if err != nil {
				t.Fatal(err)
			}
			afterStat := after.Sys().(*syscall.Stat_t)
			if beforeStat.Dev != afterStat.Dev || beforeStat.Ino != afterStat.Ino || after.Mode().Perm() != 0o600 {
				t.Errorf("target identity or mode changed: before=%v after=%v", before, after)
			}

			got = runApplicationInDirectory(t, project, []string{"XDG_CONFIG_HOME=" + configDirectory}, "", "instructions", "append", "check.md")
			if got.exitCode != 0 {
				t.Fatalf("duplicate append exit code = %d; stderr = %q", got.exitCode, got.stderr)
			}
			contents, err = os.ReadFile(targetPath)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(string(contents), test.want) || len(contents) <= len(test.want) {
				t.Errorf("duplicate append target = %q, want original result plus duplicate", contents)
			}
			if _, err := os.Lstat(filepath.Join(project, ".agents")); !os.IsNotExist(err) {
				t.Errorf("append created manifest infrastructure; Lstat error = %v", err)
			}
		})
	}
}

func TestInstructionsAppendRejectsUnsafeExistingDefaultTargets(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		setup func(string) error
	}{
		{name: "directory", setup: func(path string) error { return os.Mkdir(path, 0o755) }},
		{name: "symlink", setup: func(path string) error {
			real := filepath.Join(filepath.Dir(path), "real.md")
			if err := os.WriteFile(real, []byte("unchanged"), 0o644); err != nil {
				return err
			}
			return os.Symlink(real, path)
		}},
		{name: "hard link", setup: func(path string) error {
			real := filepath.Join(filepath.Dir(path), "real.md")
			if err := os.WriteFile(real, []byte("unchanged"), 0o644); err != nil {
				return err
			}
			return os.Link(real, path)
		}},
		{name: "invalid UTF-8", setup: func(path string) error { return os.WriteFile(path, []byte{0xff}, 0o644) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			project := t.TempDir()
			configDirectory := t.TempDir()
			writeInstructionForTest(t, configDirectory, "check.md", []byte("instruction"))
			targetPath := filepath.Join(project, "AGENTS.md")
			if err := test.setup(targetPath); err != nil {
				t.Fatal(err)
			}

			got := runApplicationInDirectory(t, project, []string{"XDG_CONFIG_HOME=" + configDirectory}, "", "instructions", "append", "check.md")
			if got.exitCode != 1 || got.stderr == "" {
				t.Errorf("exit code = %d, stderr = %q; want target validation failure", got.exitCode, got.stderr)
			}
		})
	}
}

func TestInstructionsAppendTimesOutOnCooperativelyLockedTarget(t *testing.T) {
	project := t.TempDir()
	configDirectory := t.TempDir()
	writeInstructionForTest(t, configDirectory, "check.md", []byte("instruction"))
	targetPath := filepath.Join(project, "AGENTS.md")
	if err := os.WriteFile(targetPath, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	locked, err := os.OpenFile(targetPath, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(locked.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := syscall.Flock(int(locked.Fd()), syscall.LOCK_UN); err != nil {
			t.Errorf("unlock test target: %v", err)
		}
		if err := locked.Close(); err != nil {
			t.Errorf("close test target: %v", err)
		}
	})

	got := runApplicationWithDependencies(t, project, []string{"XDG_CONFIG_HOME=" + configDirectory}, "", Dependencies{ProjectLockTimeout: 20 * time.Millisecond}, "instructions", "append", "check.md")
	if got.exitCode != 1 || !strings.Contains(got.stderr, "locked by another Bond process") {
		t.Errorf("result = exit %d, stderr %q; want lock timeout", got.exitCode, got.stderr)
	}
	contents, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "original" {
		t.Errorf("locked target = %q, want unchanged", contents)
	}
}

func TestInstructionsAppendHonorsContextCancellationWhileTargetIsLocked(t *testing.T) {
	project := t.TempDir()
	configDirectory := t.TempDir()
	writeInstructionForTest(t, configDirectory, "check.md", []byte("instruction"))
	targetPath := filepath.Join(project, "AGENTS.md")
	if err := os.WriteFile(targetPath, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	locked, err := os.OpenFile(targetPath, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(locked.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := syscall.Flock(int(locked.Fd()), syscall.LOCK_UN); err != nil {
			t.Errorf("unlock test target: %v", err)
		}
		if err := locked.Close(); err != nil {
			t.Errorf("close test target: %v", err)
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run(ctx, Invocation{
		Arguments:        []string{"instructions", "append", "check.md"},
		Environment:      []string{"XDG_CONFIG_HOME=" + configDirectory},
		WorkingDirectory: project,
		Stdout:           &stdout,
		Stderr:           &stderr,
	}, Dependencies{})
	if exitCode != 1 || !strings.Contains(stderr.String(), "context canceled") {
		t.Errorf("result = exit %d, stderr %q; want context cancellation", exitCode, stderr.String())
	}
	if stdout.String() != "" {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	contents, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "original" {
		t.Errorf("canceled append target = %q, want unchanged", contents)
	}
}

func TestInstructionsAppendRollsBackOrdinaryFailures(t *testing.T) {
	for _, test := range []struct {
		name       string
		existing   bool
		failureAt  string
		wantExists bool
	}{
		{name: "existing target after write failure", existing: true, failureAt: instructionAfterWrite, wantExists: true},
		{name: "existing target before sync failure", existing: true, failureAt: instructionBeforeSync, wantExists: true},
		{name: "new target after write failure", failureAt: instructionAfterWrite},
		{name: "new target before sync failure", failureAt: instructionBeforeSync},
	} {
		t.Run(test.name, func(t *testing.T) {
			project := t.TempDir()
			configDirectory := t.TempDir()
			writeInstructionForTest(t, configDirectory, "check.md", []byte("instruction"))
			targetPath := filepath.Join(project, "AGENTS.md")
			if test.existing {
				if err := os.WriteFile(targetPath, []byte("original"), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			got := runApplicationWithDependencies(t, project, []string{"XDG_CONFIG_HOME=" + configDirectory}, "", Dependencies{TransactionFailurePoint: test.failureAt}, "instructions", "append", "check.md")
			if got.exitCode != 1 || got.stderr == "" {
				t.Fatalf("result = exit %d, stderr %q; want injected failure", got.exitCode, got.stderr)
			}
			contents, err := os.ReadFile(targetPath)
			if test.wantExists {
				if err != nil {
					t.Fatal(err)
				}
				if string(contents) != "original" {
					t.Errorf("rolled-back target = %q, want original", contents)
				}
			} else if !os.IsNotExist(err) {
				t.Errorf("new target remains after rollback; ReadFile error = %v", err)
			}
		})
	}
}

func writeInstructionForTest(t *testing.T, configDirectory, name string, contents []byte) {
	t.Helper()

	store := filepath.Join(configDirectory, "bond", "instructions")
	if err := os.MkdirAll(store, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store, name), contents, 0o644); err != nil {
		t.Fatal(err)
	}
}
