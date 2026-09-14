package application

import (
	"bytes"
	"context"
	"fmt"
	"net"
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

func TestInstructionsAppendMultipleInstructionsInCommandLineOrder(t *testing.T) {
	project := t.TempDir()
	configDirectory := t.TempDir()
	writeInstructionForTest(t, configDirectory, "second.md", []byte("second"))
	writeInstructionForTest(t, configDirectory, "first.md", []byte("first"))

	got := runApplicationInDirectory(t, project, []string{"XDG_CONFIG_HOME=" + configDirectory}, "", "instructions", "append", "first.md", "second.md")
	if got.exitCode != 0 || got.stdout != "" || got.stderr != "" {
		t.Fatalf("result = exit %d, stdout %q, stderr %q; want silent success", got.exitCode, got.stdout, got.stderr)
	}
	contents, err := os.ReadFile(filepath.Join(project, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "first\n\nsecond\n" {
		t.Errorf("AGENTS.md = %q, want Instructions in command-line order", contents)
	}
}

func TestInstructionsAppendSupportsGroupedInstructionPaths(t *testing.T) {
	project := t.TempDir()
	configDirectory := t.TempDir()
	writeInstructionForTest(t, configDirectory, "quality/format-lint.md", []byte("Run grouped checks."))

	got := runApplicationInDirectory(t, project, []string{"XDG_CONFIG_HOME=" + configDirectory}, "", "instructions", "append", "quality/format-lint.md")
	if got.exitCode != 0 || got.stdout != "" || got.stderr != "" {
		t.Fatalf("result = exit %d, stdout %q, stderr %q; want silent success", got.exitCode, got.stdout, got.stderr)
	}
	contents, err := os.ReadFile(filepath.Join(project, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "Run grouped checks.\n" {
		t.Errorf("AGENTS.md = %q, want grouped Instruction content", contents)
	}
}

func TestInstructionsAppendSupportsCustomTargets(t *testing.T) {
	for _, test := range []struct {
		name       string
		flag       string
		target     string
		wantTarget string
	}{
		{name: "short flag and arbitrary extension", flag: "-t", target: "notes.txt", wantTarget: "notes.txt"},
		{name: "long flag and nested target", flag: "--target", target: "docs/guides/AGENT", wantTarget: "docs/guides/AGENT"},
		{name: "cleaned dot segments", flag: "--target", target: "./docs/../AGENT", wantTarget: "AGENT"},
	} {
		t.Run(test.name, func(t *testing.T) {
			project := t.TempDir()
			if err := os.MkdirAll(filepath.Join(project, "docs", "guides"), 0o755); err != nil {
				t.Fatal(err)
			}
			configDirectory := t.TempDir()
			writeInstructionForTest(t, configDirectory, "check.md", []byte("instruction"))

			got := runApplicationInDirectory(t, project, []string{"XDG_CONFIG_HOME=" + configDirectory}, "", "instructions", "append", "check.md", test.flag, test.target)
			if got.exitCode != 0 || got.stdout != "" || got.stderr != "" {
				t.Fatalf("result = exit %d, stdout %q, stderr %q; want silent success", got.exitCode, got.stdout, got.stderr)
			}
			contents, err := os.ReadFile(filepath.Join(project, filepath.FromSlash(test.wantTarget)))
			if err != nil {
				t.Fatal(err)
			}
			if string(contents) != "instruction\n" {
				t.Errorf("target = %q, want appended Instruction", contents)
			}
		})
	}
}

func TestInstructionsAppendRejectsUnsafeCustomTargetPaths(t *testing.T) {
	for _, test := range []struct {
		name   string
		target func(project, config string) string
		setup  func(*testing.T, string, string)
	}{
		{name: "absolute", target: func(_ string, _ string) string { return filepath.Join(t.TempDir(), "outside.md") }},
		{name: "traversal", target: func(_ string, _ string) string { return "../outside.md" }},
		{name: "missing parent", target: func(_ string, _ string) string { return "missing/target.md" }},
		{name: "Bond infrastructure", target: func(_ string, _ string) string { return ".agents/notes.md" }, setup: func(t *testing.T, project, _ string) {
			if err := os.Mkdir(filepath.Join(project, ".agents"), 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "case-insensitive Bond infrastructure alias", target: func(_ string, _ string) string { return ".AGENTS/notes.md" }, setup: func(t *testing.T, project, _ string) {
			if err := os.Mkdir(filepath.Join(project, ".AGENTS"), 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "Instruction Store", target: func(project, _ string) string { return "config/bond/instructions/target.md" }, setup: func(t *testing.T, project, _ string) {
			if err := os.MkdirAll(filepath.Join(project, "config", "bond", "instructions"), 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "Instruction Store case alias", target: func(project, _ string) string { return "CONFIG/BOND/INSTRUCTIONS/target.md" }, setup: func(t *testing.T, project, _ string) {
			if err := os.MkdirAll(filepath.Join(project, "CONFIG", "BOND", "INSTRUCTIONS"), 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "symlinked parent", target: func(_ string, _ string) string { return "linked/target.md" }, setup: func(t *testing.T, project, _ string) {
			if err := os.Symlink(t.TempDir(), filepath.Join(project, "linked")); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "symlinked target", target: func(_ string, _ string) string { return "target.md" }, setup: func(t *testing.T, project, _ string) {
			if err := os.Symlink(filepath.Join(t.TempDir(), "outside.md"), filepath.Join(project, "target.md")); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "hard-linked target", target: func(_ string, _ string) string { return "target.md" }, setup: func(t *testing.T, project, _ string) {
			real := filepath.Join(project, "real.md")
			if err := os.WriteFile(real, []byte("original"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Link(real, filepath.Join(project, "target.md")); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "directory target", target: func(_ string, _ string) string { return "target.md" }, setup: func(t *testing.T, project, _ string) {
			if err := os.Mkdir(filepath.Join(project, "target.md"), 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "pipe target", target: func(_ string, _ string) string { return "target.md" }, setup: func(t *testing.T, project, _ string) {
			if err := syscall.Mkfifo(filepath.Join(project, "target.md"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "socket target", target: func(_ string, _ string) string { return "target.md" }, setup: func(t *testing.T, project, _ string) {
			listener, err := net.Listen("unix", filepath.Join(project, "target.md"))
			if err != nil {
				t.Skipf("Unix socket target is not practical in this temporary directory: %v", err)
			}
			t.Cleanup(func() { _ = listener.Close() })
		}},
		{name: "invalid UTF-8 target", target: func(_ string, _ string) string { return "target.md" }, setup: func(t *testing.T, project, _ string) {
			if err := os.WriteFile(filepath.Join(project, "target.md"), []byte{0xff}, 0o644); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			project := t.TempDir()
			configDirectory := t.TempDir()
			environment := []string{"XDG_CONFIG_HOME=" + configDirectory}
			if strings.HasPrefix(test.name, "Instruction Store") {
				configDirectory = filepath.Join(project, "config")
				environment = []string{"XDG_CONFIG_HOME=" + configDirectory}
			}
			writeInstructionForTest(t, configDirectory, "check.md", []byte("instruction"))
			if test.setup != nil {
				test.setup(t, project, configDirectory)
			}

			target := test.target(project, configDirectory)
			got := runApplicationInDirectory(t, project, environment, "", "instructions", "append", "check.md", "--target", target)
			if got.exitCode != 1 || got.stdout != "" || got.stderr == "" {
				t.Errorf("result = exit %d, stdout %q, stderr %q; want one target validation failure", got.exitCode, got.stdout, got.stderr)
			}
		})
	}
}

func TestInstructionsAppendChecksManagedResourceOwnershipWithoutProjectMutation(t *testing.T) {
	for _, test := range []struct {
		name         string
		target       string
		ownedPath    string
		manifest     string
		unreadable   bool
		wantSuccess  bool
		wantContents string
	}{
		{name: "exact owned path", target: "config/tool.txt", ownedPath: "config/tool.txt"},
		{name: "beneath owned path", target: "config/tool/notes.txt", ownedPath: "config/tool"},
		{name: "ancestor of owned path", target: "config", ownedPath: "config/tool.txt"},
		{name: "case-insensitive alias of owned path", target: "Config/tool.txt", ownedPath: "config/tool.txt"},
		{name: "malformed manifest", target: "notes.txt", manifest: `{not json`},
		{name: "unreadable manifest", target: "notes.txt", manifest: `{"version":2,"skills":[],"resources":[]}`, unreadable: true},
		{name: "missing manifest", target: "notes.txt", wantSuccess: true, wantContents: "instruction\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.unreadable && os.Geteuid() == 0 {
				t.Skip("root can read mode-000 manifests")
			}
			project := t.TempDir()
			configDirectory := t.TempDir()
			writeInstructionForTest(t, configDirectory, "check.md", []byte("instruction"))
			if parent := filepath.Dir(filepath.Join(project, filepath.FromSlash(test.target))); parent != project {
				if err := os.MkdirAll(parent, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			manifest := test.manifest
			if test.ownedPath != "" {
				manifest = fmt.Sprintf(`{"version":2,"skills":[],"resources":[{"name":"tooling","mode":"copy","paths":[%q]}]}`, test.ownedPath)
			}
			manifestPath := filepath.Join(project, ".agents", "bond-manifest.json")
			if manifest != "" {
				if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
					t.Fatal(err)
				}
				if test.unreadable {
					if err := os.Chmod(manifestPath, 0); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = os.Chmod(manifestPath, 0o644) })
				}
			}

			got := runApplicationInDirectory(t, project, []string{"XDG_CONFIG_HOME=" + configDirectory}, "", "instructions", "append", "check.md", "--target", test.target)
			contents, readErr := os.ReadFile(filepath.Join(project, filepath.FromSlash(test.target)))
			if test.wantSuccess {
				if got.exitCode != 0 || got.stdout != "" || got.stderr != "" || readErr != nil || string(contents) != test.wantContents {
					t.Fatalf("result = exit %d, stdout %q, stderr %q, target %q, read error %v", got.exitCode, got.stdout, got.stderr, contents, readErr)
				}
				return
			}
			if got.exitCode != 1 || got.stdout != "" || got.stderr == "" {
				t.Errorf("result = exit %d, stdout %q, stderr %q; want ownership failure", got.exitCode, got.stdout, got.stderr)
			}
			if readErr == nil {
				t.Errorf("rejected target was changed or created: %q", contents)
			}
		})
	}
}

func TestInstructionsAppendPinsTheInvocationDirectoryBeforePreflight(t *testing.T) {
	projectParent := t.TempDir()
	project := filepath.Join(projectParent, "project")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	configDirectory := t.TempDir()
	writeInstructionForTest(t, configDirectory, "check.md", []byte("instruction"))
	var hookCalls int
	dependencies := Dependencies{transactionHook: func(point string) error {
		if point != instructionAfterPreflight {
			return nil
		}
		hookCalls++
		if err := os.Remove(project); err != nil {
			return err
		}
		return os.Symlink(outside, project)
	}}

	got := runApplicationWithDependencies(t, project, []string{"XDG_CONFIG_HOME=" + configDirectory}, "", dependencies, "instructions", "append", "check.md", "--target", "target.md")
	if got.exitCode != 1 || got.stderr == "" || hookCalls != 1 {
		t.Fatalf("result = exit %d, stderr %q, hook calls %d; want safe rejection", got.exitCode, got.stderr, hookCalls)
	}
	if _, err := os.Lstat(filepath.Join(outside, "target.md")); !os.IsNotExist(err) {
		t.Errorf("append escaped through replaced invocation directory: %v", err)
	}
}

func TestInstructionsAppendRejectsAParentMovedAfterOpening(t *testing.T) {
	project := t.TempDir()
	outside := t.TempDir()
	configDirectory := t.TempDir()
	writeInstructionForTest(t, configDirectory, "check.md", []byte("instruction"))
	parent := filepath.Join(project, "docs")
	if err := os.Mkdir(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(outside, "docs")
	var hookCalls int
	dependencies := Dependencies{transactionHook: func(point string) error {
		if point != instructionAfterParentOpen {
			return nil
		}
		hookCalls++
		return os.Rename(parent, moved)
	}}

	got := runApplicationWithDependencies(t, project, []string{"XDG_CONFIG_HOME=" + configDirectory}, "", dependencies, "instructions", "append", "check.md", "--target", "docs/target.md")
	if got.exitCode != 1 || got.stderr == "" || hookCalls != 1 {
		t.Fatalf("result = exit %d, stderr %q, hook calls %d; want safe rejection", got.exitCode, got.stderr, hookCalls)
	}
	if _, err := os.Lstat(filepath.Join(moved, "target.md")); !os.IsNotExist(err) {
		t.Errorf("append wrote through a parent moved outside the project: %v", err)
	}
}

func TestInstructionsAppendDoesNotFollowAParentChangedAfterPreflight(t *testing.T) {
	project := t.TempDir()
	outside := t.TempDir()
	configDirectory := t.TempDir()
	writeInstructionForTest(t, configDirectory, "check.md", []byte("instruction"))
	parent := filepath.Join(project, "docs")
	if err := os.Mkdir(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	var hookCalls int
	dependencies := Dependencies{transactionHook: func(point string) error {
		if point != instructionAfterPreflight {
			return nil
		}
		hookCalls++
		if err := os.Remove(parent); err != nil {
			return err
		}
		return os.Symlink(outside, parent)
	}}

	got := runApplicationWithDependencies(t, project, []string{"XDG_CONFIG_HOME=" + configDirectory}, "", dependencies, "instructions", "append", "check.md", "--target", "docs/target.md")
	if got.exitCode != 1 || got.stderr == "" || hookCalls != 1 {
		t.Fatalf("result = exit %d, stderr %q, hook calls %d; want safe rejection", got.exitCode, got.stderr, hookCalls)
	}
	if _, err := os.Lstat(filepath.Join(outside, "target.md")); !os.IsNotExist(err) {
		t.Errorf("append escaped through changed parent: %v", err)
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

func TestInstructionsAppendRequiresAtLeastOneInstructionPath(t *testing.T) {
	t.Parallel()

	arguments := []string{"instructions", "append"}
	got := runApplication(t, arguments...)
	if got.exitCode != 1 {
		t.Errorf("Run(%q) exit code = %d, want 1", arguments, got.exitCode)
	}
	if got.stdout != "" || got.stderr == "" || strings.Contains(got.stderr, "Usage:") {
		t.Errorf("Run(%q) output = stdout %q, stderr %q; want one plain diagnostic", arguments, got.stdout, got.stderr)
	}
}

func TestInstructionsAppendRejectsInvalidOrMissingSourcesWithoutCreatingFiles(t *testing.T) {
	t.Parallel()

	for _, instructionPath := range []string{
		"", "Format.md", "format_lint.md", "format-lint", "format.MD", "../format-lint.md",
		"/format-lint.md", "./format-lint.md", "group/../format-lint.md", "group//format-lint.md",
		"group/subgroup/format-lint.md", "Group/format-lint.md", "group_name/format-lint.md",
		"group/Format.md", "group/format_lint.md", "group/format--lint.md", "format--lint.md",
	} {
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

func TestInstructionsAppendSupportsSymlinkedInstructionStoreRootAndRejectsEntrySymlinks(t *testing.T) {
	for _, test := range []struct {
		name        string
		instruction string
		setup       func(t *testing.T, configuredStore, realStore string)
		wantSuccess bool
	}{
		{
			name:        "symlinked Instruction Store root",
			instruction: "quality/check.md",
			setup: func(t *testing.T, configuredStore, realStore string) {
				if err := os.MkdirAll(filepath.Join(realStore, "quality"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(realStore, "quality", "check.md"), []byte("instruction"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(filepath.Dir(configuredStore), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(realStore, configuredStore); err != nil {
					t.Fatal(err)
				}
			},
			wantSuccess: true,
		},
		{
			name:        "symlinked grouping directory",
			instruction: "quality/check.md",
			setup: func(t *testing.T, configuredStore, realStore string) {
				outside := t.TempDir()
				if err := os.WriteFile(filepath.Join(outside, "check.md"), []byte("instruction"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(configuredStore, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, filepath.Join(configuredStore, "quality")); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name:        "symlinked grouped Instruction",
			instruction: "quality/check.md",
			setup: func(t *testing.T, configuredStore, realStore string) {
				if err := os.MkdirAll(filepath.Join(configuredStore, "quality"), 0o755); err != nil {
					t.Fatal(err)
				}
				real := filepath.Join(realStore, "check.md")
				if err := os.MkdirAll(realStore, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(real, []byte("instruction"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(real, filepath.Join(configuredStore, "quality", "check.md")); err != nil {
					t.Fatal(err)
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			project := t.TempDir()
			configDirectory := t.TempDir()
			configuredStore := filepath.Join(configDirectory, "bond", "instructions")
			test.setup(t, configuredStore, t.TempDir())

			got := runApplicationInDirectory(t, project, []string{"XDG_CONFIG_HOME=" + configDirectory}, "", "instructions", "append", test.instruction)
			if test.wantSuccess {
				if got.exitCode != 0 || got.stdout != "" || got.stderr != "" {
					t.Fatalf("result = exit %d, stdout %q, stderr %q; want silent success", got.exitCode, got.stdout, got.stderr)
				}
				return
			}
			if got.exitCode != 1 || got.stdout != "" || got.stderr == "" {
				t.Errorf("result = exit %d, stdout %q, stderr %q; want entry symlink rejection", got.exitCode, got.stdout, got.stderr)
			}
			if _, err := os.Lstat(filepath.Join(project, "AGENTS.md")); !os.IsNotExist(err) {
				t.Errorf("rejected Instruction created target; Lstat error = %v", err)
			}
		})
	}
}

func TestInstructionPathCompletionReturnsOnlySafeSupportedEntries(t *testing.T) {
	project := t.TempDir()
	configDirectory := t.TempDir()
	store := filepath.Join(configDirectory, "bond", "instructions")
	writeInstructionForTest(t, configDirectory, "alpha.md", []byte("alpha"))
	writeInstructionForTest(t, configDirectory, "quality/format-lint.md", []byte("checks"))
	writeInstructionForTest(t, configDirectory, "quality/deeper/ignored.md", []byte("ignored"))
	writeInstructionForTest(t, configDirectory, "quality/not-markdown.txt", []byte("ignored"))
	writeInstructionForTest(t, configDirectory, "Bad.md", []byte("ignored"))
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "linked.md"), []byte("ignored"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "linked.md"), filepath.Join(store, "linked.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(store, "linked-group")); err != nil {
		t.Fatal(err)
	}

	got := runApplicationInDirectory(t, project, []string{"XDG_CONFIG_HOME=" + configDirectory}, "", "__complete", "instructions", "append", "")
	if got.exitCode != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", got.exitCode, got.stderr)
	}
	if got.stdout != "alpha.md\nquality/format-lint.md\n:4\n" {
		t.Errorf("stdout = %q, want valid top-level and grouped Instruction Paths", got.stdout)
	}
	if strings.Contains(got.stderr, "Error:") {
		t.Errorf("stderr = %q, want no completion diagnostic", got.stderr)
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

func TestInstructionsAppendRollbackPreservesAReplacementTarget(t *testing.T) {
	project := t.TempDir()
	configDirectory := t.TempDir()
	writeInstructionForTest(t, configDirectory, "check.md", []byte("instruction"))
	targetPath := filepath.Join(project, "target.md")
	movedPath := filepath.Join(project, "bond-created.md")
	dependencies := Dependencies{transactionHook: func(point string) error {
		if point != instructionAfterWrite {
			return nil
		}
		if err := os.Rename(targetPath, movedPath); err != nil {
			return err
		}
		if err := os.WriteFile(targetPath, []byte("replacement"), 0o600); err != nil {
			return err
		}
		return fmt.Errorf("injected replacement")
	}}

	got := runApplicationWithDependencies(t, project, []string{"XDG_CONFIG_HOME=" + configDirectory}, "", dependencies, "instructions", "append", "check.md", "--target", "target.md")
	if got.exitCode != 1 || got.stderr == "" {
		t.Fatalf("result = exit %d, stderr %q; want rollback failure", got.exitCode, got.stderr)
	}
	contents, err := os.ReadFile(targetPath)
	if err != nil || string(contents) != "replacement" {
		t.Errorf("replacement target = %q, err = %v; want preserved replacement", contents, err)
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

	path := filepath.Join(configDirectory, "bond", "instructions", filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, contents, 0o644); err != nil {
		t.Fatal(err)
	}
}
