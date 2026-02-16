package commands

import (
	"fmt"
	"path/filepath"

	"bond/internal/config"
	"bond/internal/skills"
	"github.com/spf13/cobra"
)

// newStoreCmd builds the command that stores project skills in the store directory.
func newStoreCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "store [skill ...]",
		Short: "Copy project skills into the store Bond directory",
		Args:  cobra.MinimumNArgs(1),
		RunE:  runStore,
	}

	cmd.ValidArgsFunction = completeProjectStorableSkills
	return cmd
}

// runStore executes copy operations from project-local skills to store skills.
func runStore(cmd *cobra.Command, args []string) error {
	projectSkillsDir, err := config.ProjectSkillsDir()
	if err != nil {
		return err
	}

	storeDir, err := config.StoreSkillsDir()
	if err != nil {
		return err
	}
	if _, err := ensureDir(storeDir); err != nil {
		return err
	}

	discovered, err := skills.DiscoverProjectStorable(projectSkillsDir)
	if err != nil {
		return err
	}

	return runDiscoveredSkillActions(cmd, discovered, args, func(skill skills.Skill) (skillActionOutput, error) {
		dest := filepath.Join(storeDir, skill.Name)
		result, err := skills.Copy(skill.Path, dest)
		if err != nil {
			return skillActionOutput{}, err
		}

		switch result.Status {
		case skills.CopyStatusCopied:
			return skillActionOutput{level: levelOK, message: fmt.Sprintf("stored %s", skill.Name)}, nil
		case skills.CopyStatusConflict:
			return skillActionOutput{level: levelWarn, message: fmt.Sprintf("skipped %s (already exists)", skill.Name)}, nil
		default:
			return skillActionOutput{}, fmt.Errorf("unexpected store status %q for %q", result.Status, skill.Name)
		}
	})
}

// completeProjectStorableSkills offers shell completions from project-local storable skills.
func completeProjectStorableSkills(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	projectSkillsDir, err := config.ProjectSkillsDir()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	discovered, err := skills.DiscoverProjectStorable(projectSkillsDir)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	candidates := make([]string, 0, len(discovered))
	for _, skill := range discovered {
		candidates = append(candidates, skill.Name)
	}

	return candidates, cobra.ShellCompDirectiveNoFileComp
}
