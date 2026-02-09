package commands

import (
	"fmt"
	"path/filepath"

	"bond/internal/config"
	"bond/internal/skills"
	"github.com/spf13/cobra"
)

// newLinkCmd builds the command that links global skills into the project.
func newLinkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "link [skill ...]",
		Short: "Symlink global skills into ./.agents/skills",
		RunE:  runLink,
	}

	cmd.ValidArgsFunction = completeGlobalSkills
	return cmd
}

// runLink executes link operations and prints per-skill and summary status.
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
		// This also covers the case where args were provided but none matched.
		fmt.Printf("no skills found in %s\n", sourceDir)
		return nil
	}

	var linked, skipped, conflicts, hardErrs int
	for _, skill := range selected {
		dest := filepath.Join(skillsDir, skill.Name)
		result, err := skills.Link(skill.Path, dest)
		if err != nil {
			hardErrs++
			fmt.Printf("error %s: %v\n", skill.Name, err)
			continue
		}

		switch result.Status {
		case skills.LinkStatusLinked:
			linked++
			fmt.Printf("linked %s\n", skill.Name)
		case skills.LinkStatusAlreadyLinked:
			skipped++
			fmt.Printf("already linked %s\n", skill.Name)
		case skills.LinkStatusConflict:
			conflicts++
			fmt.Printf("conflict %s\n", skill.Name)
		}
	}

	// Conflicts are soft failures that still allow the command to complete.
	fmt.Printf("summary linked=%d skipped=%d conflicts=%d errors=%d\n", linked, skipped, conflicts, hardErrs)
	if hardErrs > 0 {
		// Return non-nil only for unexpected filesystem/IO failures.
		return fmt.Errorf("link failed for %d skill(s)", hardErrs)
	}

	return nil
}

// selectSkills maps CLI args to discovered skills, preserving arg order.
func selectSkills(discovered []skills.Skill, args []string) []skills.Skill {
	if len(args) == 0 {
		return discovered
	}

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
