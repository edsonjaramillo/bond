package commands

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"bond/internal/skills"
	"github.com/spf13/cobra"
)

func TestRunDiscoveredSkillActionsReturnsNoMatchingSkillsError(t *testing.T) {
	setOutputColorMode(colorModeNever)
	setOutputShowLevel(true)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	discovered := []skills.Skill{{Name: "go", Path: "/tmp/go"}}
	err := runDiscoveredSkillActions(cmd, discovered, []string{"missing"}, func(skill skills.Skill) (skillActionOutput, error) {
		t.Fatalf("action called unexpectedly for skill %+v", skill)
		return skillActionOutput{}, nil
	})
	if err == nil {
		t.Fatal("runDiscoveredSkillActions() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "no matching skills: missing") {
		t.Fatalf("runDiscoveredSkillActions() error = %q, want no matching skills message", err)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("stdout = %q, want empty", got)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("stderr = %q, want empty", got)
	}
}

func TestRunDiscoveredSkillActionsReturnsAlreadyReportedFailureForHardErrors(t *testing.T) {
	setOutputColorMode(colorModeNever)
	setOutputShowLevel(true)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	discovered := []skills.Skill{
		{Name: "alpha", Path: "/tmp/alpha"},
		{Name: "beta", Path: "/tmp/beta"},
	}

	err := runDiscoveredSkillActions(cmd, discovered, []string{"alpha", "beta"}, func(skill skills.Skill) (skillActionOutput, error) {
		if skill.Name == "beta" {
			return skillActionOutput{}, errors.New("boom")
		}
		return skillActionOutput{
			level:   levelOK,
			message: "handled " + skill.Name,
		}, nil
	})
	if err == nil {
		t.Fatal("runDiscoveredSkillActions() error = nil, want non-nil")
	}
	if !IsAlreadyReportedFailure(err) {
		t.Fatalf("runDiscoveredSkillActions() error = %q, want already-reported failure", err)
	}

	if got := stdout.String(); got != "[OK] handled alpha\n" {
		t.Fatalf("stdout = %q, want %q", got, "[OK] handled alpha\n")
	}
	if got := stderr.String(); got != "[ERROR] beta: boom\n" {
		t.Fatalf("stderr = %q, want %q", got, "[ERROR] beta: boom\n")
	}
}
