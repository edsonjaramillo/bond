package commands

import "github.com/spf13/cobra"

// Version is the CLI version string set at build time via ldflags.
var Version = "dev"

// Execute constructs and runs the root CLI command tree.
func Execute() error {
	return newRootCmd().Execute()
}

// newRootCmd creates the top-level bond command and wires subcommands.
func newRootCmd() *cobra.Command {
	var colorFlag string

	cmd := &cobra.Command{
		Use:           "bond",
		Short:         "Manage project-local skill links",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			mode, err := parseColorMode(colorFlag)
			if err != nil {
				return err
			}
			setOutputColorMode(mode)
			return nil
		},
	}
	cmd.PersistentFlags().StringVar(&colorFlag, "color", colorModeAuto, "Colorize output: auto, always, never")

	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newLinkCmd())
	cmd.AddCommand(newCopyCmd())
	cmd.AddCommand(newStatusCmd())
	cmd.AddCommand(newUnlinkCmd())
	cmd.AddCommand(newValidateCmd())

	return cmd
}
