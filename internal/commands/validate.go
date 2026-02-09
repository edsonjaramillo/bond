package commands

import (
	"fmt"

	"bond/internal/config"
	"bond/internal/skills"
	"github.com/spf13/cobra"
)

// newValidateCmd builds the command that validates global skill metadata.
func newValidateCmd() *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "validate [skill]",
		Short: "Validate skills in the global directory",
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

	cmd.Flags().BoolVar(&all, "all", false, "Validate all discovered global skills")
	cmd.ValidArgsFunction = completeGlobalSkills
	return cmd
}

// runValidate validates one or all global skills and reports violations.
func runValidate(cmd *cobra.Command, args []string, all bool) error {
	globalDir, err := config.GlobalSkillsDir()
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	results := []skills.ValidationResult{}
	if all {
		results, err = skills.ValidateGlobalAll(globalDir)
		if err != nil {
			return err
		}
	} else {
		result, err := skills.ValidateGlobalByName(globalDir, args[0])
		if err != nil {
			return err
		}
		results = append(results, result)
	}

	var invalidSkills int
	for _, result := range results {
		if len(result.Issues) == 0 {
			if _, err := fmt.Fprintf(out, "ok %s\n", result.Name); err != nil {
				return err
			}
			continue
		}

		invalidSkills++
		if _, err := fmt.Fprintf(out, "invalid %s\n", result.Name); err != nil {
			return err
		}
		for _, issue := range result.Issues {
			if _, err := fmt.Fprintf(out, "error %s %s\n", issue.Rule, issue.Message); err != nil {
				return err
			}
		}
	}

	if invalidSkills > 0 {
		return fmt.Errorf("validation failed for %d skill(s)", invalidSkills)
	}
	return nil
}
