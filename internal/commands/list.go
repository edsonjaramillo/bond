package commands

import (
	"bond/internal/config"
	"bond/internal/skills"
	"github.com/spf13/cobra"
)

// newListCmd builds the command that lists available skills.
func newListCmd() *cobra.Command {
	var storeOnly bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List project skills or store skills",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, storeOnly)
		},
	}

	cmd.Flags().BoolVar(&storeOnly, "store", false, "List skills in the store directory")
	return cmd
}

// runList prints project skills by default, or store skills with --store.
func runList(cmd *cobra.Command, storeOnly bool) error {
	if !storeOnly {
		projectDir, err := config.ProjectSkillsDir()
		if err != nil {
			return err
		}

		discovered, err := skills.DiscoverProjectAll(projectDir)
		if err != nil {
			return err
		}

		for _, skill := range discovered {
			if err := printOut(cmd, levelInfo, "%s", skill.Name); err != nil {
				return err
			}
		}
		return nil
	}

	storeDir, err := config.StoreSkillsDir()
	if err != nil {
		return err
	}
	discovered, err := skills.Discover(storeDir)
	if err != nil {
		return err
	}

	for _, skill := range discovered {
		if err := printOut(cmd, levelInfo, "%s", skill.Name); err != nil {
			return err
		}
	}
	return nil
}
