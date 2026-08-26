package application

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

type result struct {
	exitCode int
	stdout   string
	stderr   string
}

func runApplication(t *testing.T, arguments ...string) result {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run(context.Background(), Invocation{
		Arguments:        arguments,
		Environment:      []string{"HOME=/home/tester"},
		WorkingDirectory: "/project",
		Stdin:            strings.NewReader(""),
		Stdout:           &stdout,
		Stderr:           &stderr,
	}, Dependencies{})

	return result{
		exitCode: exitCode,
		stdout:   stdout.String(),
		stderr:   stderr.String(),
	}
}

func TestBareCommandsPrintHelp(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		arguments []string
		usage     string
	}{
		{name: "root", arguments: nil, usage: "Usage:\n  bond"},
		{name: "skills", arguments: []string{"skills"}, usage: "Usage:\n  bond skills"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := runApplication(t, test.arguments...)
			if got.exitCode != 0 {
				t.Fatalf("exit code = %d, want 0; stderr = %q", got.exitCode, got.stderr)
			}
			if !strings.Contains(got.stdout, test.usage) {
				t.Errorf("stdout = %q, want help containing %q", got.stdout, test.usage)
			}
			if got.stderr != "" {
				t.Errorf("stderr = %q, want empty", got.stderr)
			}
		})
	}
}

func TestVersionInterfacesPrintOnlyVersion(t *testing.T) {
	t.Parallel()

	for _, arguments := range [][]string{{"version"}, {"--version"}} {
		got := runApplication(t, arguments...)
		if got.exitCode != 0 {
			t.Fatalf("Run(%q) exit code = %d, want 0; stderr = %q", arguments, got.exitCode, got.stderr)
		}
		if got.stdout != "dev\n" {
			t.Errorf("Run(%q) stdout = %q, want %q", arguments, got.stdout, "dev\n")
		}
		if got.stderr != "" {
			t.Errorf("Run(%q) stderr = %q, want empty", arguments, got.stderr)
		}
	}
}

func TestVersionDependencyCanBeReplaced(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run(context.Background(), Invocation{
		Arguments: []string{"version"},
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
	}, Dependencies{Version: "v1.2.3"})

	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}
	if got := stdout.String(); got != "v1.2.3\n" {
		t.Errorf("stdout = %q, want %q", got, "v1.2.3\n")
	}
}

func TestCompletionScripts(t *testing.T) {
	t.Parallel()

	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		t.Run(shell, func(t *testing.T) {
			t.Parallel()

			got := runApplication(t, "completion", shell)
			if got.exitCode != 0 {
				t.Fatalf("exit code = %d, want 0; stderr = %q", got.exitCode, got.stderr)
			}
			if strings.TrimSpace(got.stdout) == "" {
				t.Error("stdout is empty, want a native completion script")
			}
			if got.stderr != "" {
				t.Errorf("stderr = %q, want empty", got.stderr)
			}
		})
	}
}

func TestFailuresUsePlainStderrWithoutUsage(t *testing.T) {
	t.Parallel()

	got := runApplication(t, "--not-a-real-flag")

	if got.exitCode != 1 {
		t.Errorf("exit code = %d, want 1", got.exitCode)
	}
	if got.stdout != "" {
		t.Errorf("stdout = %q, want empty", got.stdout)
	}
	if got.stderr != "unknown flag: --not-a-real-flag\n" {
		t.Errorf("stderr = %q, want concise plain-text diagnostic", got.stderr)
	}
	for _, unwanted := range []string{"Usage:", "Error:", "goroutine", "github.com/"} {
		if strings.Contains(got.stderr, unwanted) {
			t.Errorf("stderr contains %q: %q", unwanted, got.stderr)
		}
	}
}
