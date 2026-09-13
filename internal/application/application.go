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
	transactionHook              func(string) error
}

// Dependencies contains replaceable values supplied by the executable.
type Dependencies struct {
	Version                      string
	ProjectLockTimeout           time.Duration
	TransactionFailurePoint      string
	TransactionInterruptionPoint string
	transactionHook              func(string) error
}

// Run executes one Bond invocation and returns its process exit code.
func Run(ctx context.Context, invocation Invocation, dependencies Dependencies) int {
	invocation = withDefaultStreams(invocation)
	invocation.projectLockTimeout = dependencies.ProjectLockTimeout
	invocation.transactionFailurePoint = dependencies.TransactionFailurePoint
	invocation.transactionInterruptionPoint = dependencies.TransactionInterruptionPoint
	invocation.transactionHook = dependencies.transactionHook
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
		Short:         "Manage reusable AI-agent skills and project resources",
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
	root.AddCommand(newResourcesCommand(invocation))
	root.AddCommand(newInstructionsCommand(invocation))
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
	command.AddCommand(newClearCommand(invocation))
	command.AddCommand(newEditCommand(invocation))

	return command
}

func newResourcesCommand(invocation Invocation) *cobra.Command {
	command := &cobra.Command{
		Use:   "resources",
		Short: "Manage resources",
		Args:  cobra.NoArgs,
		RunE:  showHelp,
	}
	command.AddCommand(newResourceAddCommand(invocation))
	command.AddCommand(newResourceRemoveCommand(invocation))

	return command
}

func newInstructionsCommand(invocation Invocation) *cobra.Command {
	command := &cobra.Command{
		Use:   "instructions",
		Short: "Manage Instructions",
		Args:  cobra.NoArgs,
		RunE:  showHelp,
	}
	command.AddCommand(newInstructionAppendCommand(invocation))

	return command
}

func newInstructionAppendCommand(invocation Invocation) *cobra.Command {
	target := "AGENTS.md"
	command := &cobra.Command{
		Use:   "append <instruction-path>",
		Short: "Append a Stored Instruction",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, arguments []string) error {
			return appendInstruction(command, invocation, arguments[0], target)
		},
	}
	command.Flags().StringVarP(&target, "target", "t", target, "append to a project-relative target")

	return command
}

func newResourceAddCommand(invocation Invocation) *cobra.Command {
	var copyResources bool
	command := &cobra.Command{
		Use:               "add <resource>...",
		Short:             "Install Stored Resources",
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: completeStoredResources(invocation),
		RunE: func(command *cobra.Command, arguments []string) error {
			mode := linkMode
			if copyResources {
				mode = copyMode
			}

			return addResources(command, invocation, arguments, mode)
		},
	}
	command.Flags().BoolVar(&copyResources, "copy", false, "install independent copies")

	return command
}

func newResourceRemoveCommand(invocation Invocation) *cobra.Command {
	return &cobra.Command{
		Use:               "remove <resource>...",
		Short:             "Remove Managed Resources",
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: completeManagedResources(invocation),
		RunE: func(command *cobra.Command, arguments []string) error {
			return removeResources(command, invocation, arguments)
		},
	}
}

func newSkillDraftCommand(invocation Invocation) *cobra.Command {
	return &cobra.Command{
		Use:               "new <stored-path>",
		Short:             "Create a Skill Draft",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeSkillDraftOrganizations(invocation),
		RunE: func(command *cobra.Command, arguments []string) error {
			return newSkillDraft(command, invocation, arguments[0])
		},
	}
}

func newAddCommand(invocation Invocation) *cobra.Command {
	var copySkills bool

	command := &cobra.Command{
		Use:               "add <stored-path>...",
		Short:             "Install Stored Skills",
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: completeStoredSkills(invocation),
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
		Use:               "remove <skill-name>...",
		Short:             "Remove Managed Skills",
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: completeManagedSkills(invocation),
		RunE: func(command *cobra.Command, arguments []string) error {
			return removeSkills(command, invocation, arguments)
		},
	}
}

func newClearCommand(invocation Invocation) *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Remove all Managed Skills",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return clearSkills(command, invocation)
		},
	}
}

func newEditCommand(invocation Invocation) *cobra.Command {
	return &cobra.Command{
		Use:               "edit <skill-name>",
		Short:             "Edit a Project Skill",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeProjectSkills(invocation),
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
