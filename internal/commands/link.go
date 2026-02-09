package commands

import (
	"fmt"
	"path/filepath"
	"strings"

	"bond/internal/config"
	"bond/internal/skills"
	"github.com/spf13/cobra"
)

// newLinkCmd builds the command that links global skills into the project.
func newLinkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "link [skill ...]",
		Short: "Symlink global skills into ./.agents/skills",
		Args:  cobra.MinimumNArgs(1),
		RunE:  runLink,
	}

	cmd.ValidArgsFunction = completeGlobalSkills
	return cmd
}

// runLink executes link operations and prints per-skill status.
func runLink(cmd *cobra.Command, args []string) error {
	sourceDir, err := config.GlobalSkillsDir()
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

	selected := selectSkills(discovered, args)
	if len(selected) == 0 {
		return fmt.Errorf("no matching skills: %s", strings.Join(args, ", "))
	}

	var hardErrs int
	for _, skill := range selected {
		dest := filepath.Join(skillsDir, skill.Name)
		result, err := skills.Link(skill.Path, dest)
		if err != nil {
			hardErrs++
			if printErrErr := printErr(cmd, levelError, "%s: %v", skill.Name, err); printErrErr != nil {
				return printErrErr
			}
			continue
		}

		switch result.Status {
		case skills.LinkStatusLinked:
			if err := printOut(cmd, levelOK, "linked %s", skill.Name); err != nil {
				return err
			}
		case skills.LinkStatusAlreadyLinked:
			if err := printOut(cmd, levelInfo, "already linked %s", skill.Name); err != nil {
				return err
			}
		case skills.LinkStatusConflict:
			if err := printOut(cmd, levelWarn, "conflict %s", skill.Name); err != nil {
				return err
			}
		}
	}

	if hardErrs > 0 {
		return alreadyReportedFailure()
	}

	return nil
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

// completeGlobalSkills offers shell completions from globally discovered skills.
func completeGlobalSkills(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	sourceDir, err := config.GlobalSkillsDir()
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
