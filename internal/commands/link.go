package commands

import (
	"fmt"
	"path/filepath"

	"bond/internal/config"
	"bond/internal/skills"
	"github.com/spf13/cobra"
)

// newLinkCmd builds the command that links store skills into the project.
func newLinkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "link [skill ...]",
		Short: "Symlink store skills into ./.agents/skills",
		Args:  cobra.MinimumNArgs(1),
		RunE:  runLink,
	}

	cmd.ValidArgsFunction = completeStoreSkills
	return cmd
}

// runLink executes link operations and prints per-skill status.
func runLink(cmd *cobra.Command, args []string) error {
	sourceDir, err := config.StoreSkillsDir()
	if err != nil {
		return err
	}

	skillsDir, err := config.ProjectSkillsDir()
	if err != nil {
		return err
	}
	if _, err := ensureDir(skillsDir); err != nil {
		return err
	}

	discovered, err := skills.Discover(sourceDir)
	if err != nil {
		return err
	}

	return runDiscoveredSkillActions(cmd, discovered, args, func(skill skills.Skill) (skillActionOutput, error) {
		dest := filepath.Join(skillsDir, skill.Name)
		result, err := skills.Link(skill.Path, dest)
		if err != nil {
			return skillActionOutput{}, err
		}

		switch result.Status {
		case skills.LinkStatusLinked:
			return skillActionOutput{level: levelOK, message: fmt.Sprintf("linked %s", skill.Name)}, nil
		case skills.LinkStatusAlreadyLinked:
			return skillActionOutput{level: levelInfo, message: fmt.Sprintf("already linked %s", skill.Name)}, nil
		case skills.LinkStatusConflict:
			return skillActionOutput{level: levelWarn, message: fmt.Sprintf("conflict %s", skill.Name)}, nil
		default:
			return skillActionOutput{}, fmt.Errorf("unexpected link status %q for %q", result.Status, skill.Name)
		}
	})
}

// selectSkills maps CLI args to discovered skills, preserving arg order.
func selectSkills(discovered []skills.Skill, args []string) []skills.Skill {
	byName := make(map[string]skills.Skill, len(discovered))
	for _, skill := range discovered {
		byName[skill.Name] = skill
	}

	selected := make([]skills.Skill, 0, len(args))
	for _, name := range args {
		// Unknown names are ignored so completion and manual input behave the same.
		if skill, ok := byName[name]; ok {
			selected = append(selected, skill)
		}
	}
	return selected
}

// completeStoreSkills offers shell completions from discovered store skills.
func completeStoreSkills(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	sourceDir, err := config.StoreSkillsDir()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	discovered, err := skills.Discover(sourceDir)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	candidates := make([]string, 0, len(discovered))
	for _, skill := range discovered {
		candidates = append(candidates, skill.Name)
	}

	return candidates, cobra.ShellCompDirectiveNoFileComp
}
