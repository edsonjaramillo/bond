package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"bond/internal/config"
	"bond/internal/skills"
	"github.com/spf13/cobra"
)

// newEditCmd builds the command that opens a store skill in the configured editor.
func newEditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <skill>",
		Short: "Open a store skill SKILL.md in $EDITOR",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEdit(cmd, args[0])
		},
	}

	cmd.ValidArgsFunction = completeStoreSkills
	return cmd
}

// runEdit resolves one store skill and opens its SKILL.md in the configured editor.
func runEdit(cmd *cobra.Command, name string) error {
	editor := strings.TrimSpace(os.Getenv("EDITOR"))
	if editor == "" {
		return fmt.Errorf("EDITOR environment variable is not set")
	}

	storeDir, err := config.StoreSkillsDir()
	if err != nil {
		return err
	}

	discovered, err := skills.Discover(storeDir)
	if err != nil {
		return err
	}

	selected := selectSkills(discovered, []string{name})
	if len(selected) == 0 {
		return fmt.Errorf("no matching skills: %s", name)
	}

	skillFile := filepath.Join(selected[0].Path, "SKILL.md")

	// Use shell parsing so EDITOR values like "code -w" work as expected.
	editCmd := exec.Command("sh", "-c", editor+" \"$1\"", "bond-edit", skillFile)
	editCmd.Stdin = cmd.InOrStdin()
	editCmd.Stdout = cmd.OutOrStdout()
	editCmd.Stderr = cmd.ErrOrStderr()

	if err := editCmd.Run(); err != nil {
		return fmt.Errorf("failed to open editor for %q: %w", name, err)
	}
	return nil
}
