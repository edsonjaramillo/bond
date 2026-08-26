// Package application provides Bond's command-line application boundary.
package application

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

// Version is the build version and may be replaced at link time.
var Version = "dev"

// Invocation contains the process state visible to one application run.
type Invocation struct {
	Arguments        []string
	Environment      []string
	WorkingDirectory string
	Stdin            io.Reader
	Stdout           io.Writer
	Stderr           io.Writer
}

// Dependencies contains replaceable values supplied by the executable.
type Dependencies struct {
	Version string
}

// Run executes one Bond invocation and returns its process exit code.
func Run(ctx context.Context, invocation Invocation, dependencies Dependencies) int {
	invocation = withDefaultStreams(invocation)
	version := dependencies.Version
	if version == "" {
		version = Version
	}

	command := newRootCommand(invocation, version)
	command.SetArgs(invocation.Arguments)
	command.SetContext(ctx)

	if err := command.Execute(); err != nil {
		_, _ = fmt.Fprintln(invocation.Stderr, err)

		return 1
	}

	return 0
}

func withDefaultStreams(invocation Invocation) Invocation {
	if invocation.Stdin == nil {
		invocation.Stdin = strings.NewReader("")
	}
	if invocation.Stdout == nil {
		invocation.Stdout = io.Discard
	}
	if invocation.Stderr == nil {
		invocation.Stderr = io.Discard
	}

	return invocation
}

func newRootCommand(invocation Invocation, version string) *cobra.Command {
	root := &cobra.Command{
		Use:           "bond",
		Short:         "Manage reusable AI-agent skills",
		SilenceErrors: true,
		SilenceUsage:  true,
		Version:       version,
		Args:          cobra.NoArgs,
		RunE:          showHelp,
	}
	root.SetVersionTemplate("{{.Version}}\n")
	root.SetIn(invocation.Stdin)
	root.SetOut(invocation.Stdout)
	root.SetErr(invocation.Stderr)

	root.AddCommand(newSkillsCommand(invocation))
	root.AddCommand(newVersionCommand(version))

	return root
}

func newSkillsCommand(invocation Invocation) *cobra.Command {
	command := &cobra.Command{
		Use:   "skills",
		Short: "Manage skills",
		Args:  cobra.NoArgs,
		RunE:  showHelp,
	}
	command.AddCommand(newListCommand(invocation))

	return command
}

func newListCommand(invocation Invocation) *cobra.Command {
	var store bool

	command := &cobra.Command{
		Use:   "list",
		Short: "List skills",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if !store {
				return printNoSkillsFound(command.OutOrStdout())
			}

			return listStoredSkills(command, invocation.Environment)
		},
	}
	command.Flags().BoolVar(&store, "store", false, "list Stored Skills")

	return command
}

func newVersionCommand(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the Bond version",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			_, err := fmt.Fprintln(command.OutOrStdout(), version)

			return err
		},
	}
}

func showHelp(command *cobra.Command, _ []string) error {
	return command.Help()
}
