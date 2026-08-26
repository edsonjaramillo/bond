// Package application provides Bond's command-line application boundary.
package application

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// Version is the build version and may be replaced at link time.
var Version = "dev"

// Invocation contains the process state visible to one application run.
type Invocation struct {
	Arguments                    []string
	Environment                  []string
	WorkingDirectory             string
	Stdin                        io.Reader
	Stdout                       io.Writer
	Stderr                       io.Writer
	projectLockTimeout           time.Duration
	transactionFailurePoint      string
	transactionInterruptionPoint string
}

// Dependencies contains replaceable values supplied by the executable.
type Dependencies struct {
	Version                      string
	ProjectLockTimeout           time.Duration
	TransactionFailurePoint      string
	TransactionInterruptionPoint string
}

// Run executes one Bond invocation and returns its process exit code.
func Run(ctx context.Context, invocation Invocation, dependencies Dependencies) int {
	invocation = withDefaultStreams(invocation)
	invocation.projectLockTimeout = dependencies.ProjectLockTimeout
	invocation.transactionFailurePoint = dependencies.TransactionFailurePoint
	invocation.transactionInterruptionPoint = dependencies.TransactionInterruptionPoint
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
	command.AddCommand(newSkillDraftCommand(invocation))
	command.AddCommand(newAddCommand(invocation))
	command.AddCommand(newRemoveCommand(invocation))
	command.AddCommand(newEditCommand(invocation))

	return command
}

func newSkillDraftCommand(invocation Invocation) *cobra.Command {
	return &cobra.Command{
		Use:   "new <stored-path>",
		Short: "Create a Skill Draft",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, arguments []string) error {
			return newSkillDraft(command, invocation, arguments[0])
		},
	}
}

func newAddCommand(invocation Invocation) *cobra.Command {
	var copySkills bool

	command := &cobra.Command{
		Use:   "add <stored-path>...",
		Short: "Install Stored Skills",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(command *cobra.Command, arguments []string) error {
			mode := linkMode
			if copySkills {
				mode = copyMode
			}

			return addSkills(command, invocation, arguments, mode)
		},
	}
	command.Flags().BoolVar(&copySkills, "copy", false, "install independent copies")

	return command
}

func newRemoveCommand(invocation Invocation) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <skill-name>...",
		Short: "Remove Managed Skills",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(command *cobra.Command, arguments []string) error {
			return removeSkills(command, invocation, arguments)
		},
	}
}

func newEditCommand(invocation Invocation) *cobra.Command {
	return &cobra.Command{
		Use:   "edit <skill-name>",
		Short: "Edit a Project Skill",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, arguments []string) error {
			return editProjectSkill(command, invocation, arguments[0])
		},
	}
}

func newListCommand(invocation Invocation) *cobra.Command {
	var store bool

	command := &cobra.Command{
		Use:   "list",
		Short: "List skills",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if store {
				return listStoredSkills(command, invocation.Environment)
			}

			return listProjectSkills(command, invocation)
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
