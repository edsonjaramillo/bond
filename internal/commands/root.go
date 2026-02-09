package commands

import "github.com/spf13/cobra"

// Execute constructs and runs the root CLI command tree.
func Execute() error {
	return newRootCmd().Execute()
}

// newRootCmd creates the top-level bond command and wires subcommands.
func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "bond",
		Short:         "Manage project-local skill links",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newLinkCmd())
	cmd.AddCommand(newStatusCmd())
	cmd.AddCommand(newUnlinkCmd())

	return cmd
}
