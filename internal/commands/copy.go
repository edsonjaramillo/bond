package commands

import (
	"fmt"
	"path/filepath"
	"strings"

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

	selected := selectSkills(discovered, args)
	if len(selected) == 0 {
		return fmt.Errorf("no matching skills: %s", strings.Join(args, ", "))
	}

	var hardErrs int
	for _, skill := range selected {
		dest := filepath.Join(skillsDir, skill.Name)
		result, err := skills.Copy(skill.Path, dest)
		if err != nil {
			hardErrs++
			if printErrErr := printErr(cmd, levelError, "%s: %v", skill.Name, err); printErrErr != nil {
				return printErrErr
			}
			continue
		}

		switch result.Status {
		case skills.CopyStatusCopied:
			if err := printOut(cmd, levelOK, "copied %s", skill.Name); err != nil {
				return err
			}
		case skills.CopyStatusConflict:
			if err := printOut(cmd, levelWarn, "skipped %s (already exists)", skill.Name); err != nil {
				return err
			}
		}
	}

	if hardErrs > 0 {
		return alreadyReportedFailure()
	}

	return nil
}
