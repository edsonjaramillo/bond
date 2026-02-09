package commands

import (
	"fmt"

	"bond/internal/config"
	"bond/internal/skills"
	"github.com/spf13/cobra"
)

// newListCmd builds the command that lists available skills.
func newListCmd() *cobra.Command {
	var projectOnly bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List global skills or project-linked skills",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, projectOnly)
		},
	}

	cmd.Flags().BoolVar(&projectOnly, "project", false, "List skills linked in the current project")
	return cmd
}

// runList prints global skills by default, or project-linked global skills.
func runList(cmd *cobra.Command, projectOnly bool) error {
	globalDir, err := config.GlobalSkillsDir()
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	if !projectOnly {
		discovered, err := skills.Discover(globalDir)
		if err != nil {
			return err
		}

		for _, skill := range discovered {
			fmt.Fprintln(out, skill.Name)
		}
		fmt.Fprintf(out, "summary total=%d\n", len(discovered))
		return nil
	}

	projectDir, err := config.ProjectSkillsDir()
	if err != nil {
		return err
	}

	linked, err := skills.DiscoverProjectLinkedGlobal(projectDir, globalDir)
	if err != nil {
		return err
	}

	for _, entry := range linked {
		fmt.Fprintln(out, entry.Name)
	}
	fmt.Fprintf(out, "summary linked=%d\n", len(linked))
	return nil
}
