package commands

import (
	"fmt"
	"strings"

	"bond/internal/skills"
	"github.com/spf13/cobra"
)

type skillActionOutput struct {
	level   string
	message string
}

// runDiscoveredSkillActions maps args to discovered skills and executes one action per match.
func runDiscoveredSkillActions(
	cmd *cobra.Command,
	discovered []skills.Skill,
	args []string,
	action func(skill skills.Skill) (skillActionOutput, error),
) error {
	selected := selectSkills(discovered, args)
	if len(selected) == 0 {
		return fmt.Errorf("no matching skills: %s", strings.Join(args, ", "))
	}

	var hardErrs int
	for _, skill := range selected {
		output, err := action(skill)
		if err != nil {
			hardErrs++
			if printErrErr := printErr(cmd, levelError, "%s: %v", skill.Name, err); printErrErr != nil {
				return printErrErr
			}
			continue
		}

		if err := printOut(cmd, output.level, "%s", output.message); err != nil {
			return err
		}
	}

	if hardErrs > 0 {
		return alreadyReportedFailure()
	}
	return nil
}
