package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bond/internal/skills"
)

func TestCreateCommandRequiresExactlyOneSkillNameArg(t *testing.T) {
	cmd := newCreateCmd()
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want arg validation error")
	}
	if !strings.Contains(err.Error(), "accepts 1 arg(s), received 0") {
		t.Fatalf("Execute() error = %q, want exact-args error", err)
	}
}

func TestCreateCommandCreatesSkillWithDefaultDescription(t *testing.T) {
	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "project")
	xdgConfig := filepath.Join(tmp, "xdg")
	storeDir := filepath.Join(xdgConfig, "bond")
	skillDir := filepath.Join(storeDir, "go")

	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(projectRoot) error = %v", err)
	}
	chdirForTest(t, projectRoot)
	t.Setenv("XDG_CONFIG_HOME", xdgConfig)

	buf := &bytes.Buffer{}
	cmd := newCreateCmd()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"go"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if got := buf.String(); got != "[OK] created go\n[WARN] add a description that describes the skill\n" {
		t.Fatalf("output = %q, want %q", got, "[OK] created go\n[WARN] add a description that describes the skill\n")
	}

	raw, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("ReadFile(SKILL.md) error = %v", err)
	}
	want := "---\nname: go\ndescription: \"TODO: describe this skill\"\n---\n"
	if got := string(raw); got != want {
		t.Fatalf("SKILL.md contents = %q, want %q", got, want)
	}

	result, err := skills.ValidateSkillDir(skillDir)
	if err != nil {
		t.Fatalf("ValidateSkillDir() error = %v", err)
	}
	if len(result.Issues) != 0 {
		t.Fatalf("len(result.Issues) = %d, want 0", len(result.Issues))
	}
}

func TestCreateCommandUsesCustomDescription(t *testing.T) {
	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "project")
	xdgConfig := filepath.Join(tmp, "xdg")
	skillDir := filepath.Join(xdgConfig, "bond", "api")

	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(projectRoot) error = %v", err)
	}
	chdirForTest(t, projectRoot)
	t.Setenv("XDG_CONFIG_HOME", xdgConfig)

	buf := &bytes.Buffer{}
	cmd := newCreateCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"api", "--description", "Web API helpers"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("ReadFile(SKILL.md) error = %v", err)
	}
	if got := string(raw); !strings.Contains(got, "description: \"Web API helpers\"\n") {
		t.Fatalf("SKILL.md contents missing custom description: %q", got)
	}
	if strings.Contains(buf.String(), "[WARN] add a description that describes the skill\n") {
		t.Fatalf("output contains unexpected warning: %q", buf.String())
	}
}

func TestCreateCommandReturnsErrorWhenSkillAlreadyExists(t *testing.T) {
	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "project")
	xdgConfig := filepath.Join(tmp, "xdg")
	skillDir := filepath.Join(xdgConfig, "bond", "go")

	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(projectRoot) error = %v", err)
	}
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(skillDir) error = %v", err)
	}
	chdirForTest(t, projectRoot)
	t.Setenv("XDG_CONFIG_HOME", xdgConfig)

	buf := &bytes.Buffer{}
	cmd := newCreateCmd()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"go"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), `already exists`) {
		t.Fatalf("Execute() error = %q, want exists error", err)
	}
	if got := buf.String(); got != "" {
		t.Fatalf("output = %q, want empty output", got)
	}
}

func TestCreateCommandRejectsInvalidSkillNames(t *testing.T) {
	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "project")
	xdgConfig := filepath.Join(tmp, "xdg")

	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(projectRoot) error = %v", err)
	}
	chdirForTest(t, projectRoot)
	t.Setenv("XDG_CONFIG_HOME", xdgConfig)

	tests := []struct {
		name    string
		skill   string
		wantErr string
	}{
		{name: "uppercase", skill: "Go", wantErr: "must use lowercase letters"},
		{name: "double hyphen", skill: "go--api", wantErr: "must use lowercase letters"},
		{name: "too long", skill: strings.Repeat("a", 65), wantErr: "maximum is 64"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newCreateCmd()
			cmd.SetArgs([]string{tc.skill})

			err := cmd.Execute()
			if err == nil {
				t.Fatal("Execute() error = nil, want non-nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Execute() error = %q, want substring %q", err, tc.wantErr)
			}
		})
	}
}
