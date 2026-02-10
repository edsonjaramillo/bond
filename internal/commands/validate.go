package commands

import (
	"fmt"

	"bond/internal/config"
	"bond/internal/skills"
	"github.com/spf13/cobra"
)

// newValidateCmd builds the command that validates store skill metadata.
func newValidateCmd() *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "validate [skill]",
		Short: "Validate skills in the store directory",
		Args: func(cmd *cobra.Command, args []string) error {
			if all {
				if len(args) > 0 {
					return fmt.Errorf("--all validates every skill and cannot be combined with a specific skill name")
				}
				return nil
			}
			if len(args) != 1 {
				return fmt.Errorf("provide exactly one skill name, or pass --all to validate every skill")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runValidate(cmd, args, all)
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Validate all discovered store skills")
	cmd.ValidArgsFunction = completeStoreSkills
	return cmd
}

// runValidate validates one or all store skills and reports violations.
func runValidate(cmd *cobra.Command, args []string, all bool) error {
	storeDir, err := config.StoreSkillsDir()
	if err != nil {
		return err
	}

	results := []skills.ValidationResult{}
	if all {
		results, err = skills.ValidateStoreAll(storeDir)
		if err != nil {
			return err
		}
	} else {
		result, err := skills.ValidateStoreByName(storeDir, args[0])
		if err != nil {
			return err
		}
		results = append(results, result)
	}

	var invalidSkills int
	for _, result := range results {
		if len(result.Issues) == 0 {
			if err := printOut(cmd, levelOK, "%s", result.Name); err != nil {
				return err
			}
			continue
		}

		invalidSkills++
		for _, issue := range result.Issues {
			if err := printOut(cmd, levelError, "(%s) %s: %s", result.Name, issue.Rule, issue.Message); err != nil {
				return err
			}
		}
	}

	if invalidSkills > 0 {
		return alreadyReportedFailure()
	}
	return nil
}
