package commands

import (
	"path/filepath"
	"strings"
	"testing"

	"bond/internal/skills"
)

func TestLinkCommandRequiresAtLeastOneSkillArg(t *testing.T) {
	cmd := newLinkCmd()
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want arg validation error")
	}
	if !strings.Contains(err.Error(), "requires at least 1 arg(s)") {
		t.Fatalf("Execute() error = %q, want minimum args error", err)
	}
}

func TestUnlinkCommandRequiresAtLeastOneSkillArg(t *testing.T) {
	cmd := newUnlinkCmd()
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want arg validation error")
	}
	if !strings.Contains(err.Error(), "requires at least 1 arg(s)") {
		t.Fatalf("Execute() error = %q, want minimum args error", err)
	}
}

func TestSelectSkillsUsesOnlyExplicitArgs(t *testing.T) {
	discovered := []skills.Skill{
		{Name: "alpha", Path: "/tmp/alpha"},
		{Name: "beta", Path: "/tmp/beta"},
		{Name: "gamma", Path: "/tmp/gamma"},
	}

	selected := selectSkills(discovered, []string{"gamma", "alpha", "missing"})
	if len(selected) != 2 {
		t.Fatalf("len(selected) = %d, want 2", len(selected))
	}
	if selected[0].Name != "gamma" {
		t.Fatalf("selected[0].Name = %q, want gamma", selected[0].Name)
	}
	if selected[1].Name != "alpha" {
		t.Fatalf("selected[1].Name = %q, want alpha", selected[1].Name)
	}

	none := selectSkills(discovered, nil)
	if len(none) != 0 {
		t.Fatalf("len(selectSkills(discovered, nil)) = %d, want 0", len(none))
	}
}

func TestResolveUnlinkTargetsBuildsOnlyFromArgs(t *testing.T) {
	skillsDir := filepath.Join("/tmp", "project", ".agents", "skills")

	entries, err := resolveUnlinkTargets(skillsDir, []string{"one", "two"})
	if err != nil {
		t.Fatalf("resolveUnlinkTargets() error = %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	if entries[0].Name != "one" || entries[0].Path != filepath.Join(skillsDir, "one") {
		t.Fatalf("entries[0] = %+v, want name/path for one", entries[0])
	}
	if entries[1].Name != "two" || entries[1].Path != filepath.Join(skillsDir, "two") {
		t.Fatalf("entries[1] = %+v, want name/path for two", entries[1])
	}

	none, err := resolveUnlinkTargets(skillsDir, nil)
	if err != nil {
		t.Fatalf("resolveUnlinkTargets(nil args) error = %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("len(resolveUnlinkTargets(nil args)) = %d, want 0", len(none))
	}
}
