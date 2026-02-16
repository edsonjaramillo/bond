package commands

import (
	"fmt"
	"path/filepath"

	"bond/internal/config"
	"bond/internal/skills"
	"github.com/spf13/cobra"
)

// newCopyCmd builds the command that copies store skills into the project.
func newCopyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "copy [skill ...]",
		Short: "Copy store skills into ./.agents/skills",
		Args:  cobra.MinimumNArgs(1),
		RunE:  runCopy,
	}

	cmd.ValidArgsFunction = completeStoreSkills
	return cmd
}

// runCopy executes copy operations and prints per-skill status.
func runCopy(cmd *cobra.Command, args []string) error {
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
		result, err := skills.Copy(skill.Path, dest)
		if err != nil {
			return skillActionOutput{}, err
		}

		switch result.Status {
		case skills.CopyStatusCopied:
			return skillActionOutput{level: levelOK, message: fmt.Sprintf("copied %s", skill.Name)}, nil
		case skills.CopyStatusConflict:
			return skillActionOutput{level: levelWarn, message: fmt.Sprintf("skipped %s (already exists)", skill.Name)}, nil
		default:
			return skillActionOutput{}, fmt.Errorf("unexpected copy status %q for %q", result.Status, skill.Name)
		}
	})
}
