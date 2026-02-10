package commands

import (
	"fmt"
	"path/filepath"
	"strings"

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

	selected := selectSkills(discovered, args)
	if len(selected) == 0 {
		return fmt.Errorf("no matching skills: %s", strings.Join(args, ", "))
	}

	var hardErrs int
	for _, skill := range selected {
		dest := filepath.Join(storeDir, skill.Name)
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
			if err := printOut(cmd, levelOK, "stored %s", skill.Name); err != nil {
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
