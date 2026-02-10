package commands

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"bond/internal/config"
	"bond/internal/skills"
	"github.com/spf13/cobra"
)

// newEditCmd builds the command that opens a store skill marker in the user's editor.
func newEditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit [skill]",
		Short: "Open a store skill SKILL.md in $EDITOR",
		Args:  cobra.ExactArgs(1),
		RunE:  runEdit,
	}

	cmd.ValidArgsFunction = completeStoreSkills
	return cmd
}

// runEdit resolves a store skill and opens its SKILL.md with the configured editor.
func runEdit(cmd *cobra.Command, args []string) error {
	editor := strings.TrimSpace(os.Getenv("EDITOR"))
	if editor == "" {
		return errors.New("EDITOR is not set")
	}

	storeDir, err := config.StoreSkillsDir()
	if err != nil {
		return err
	}

	discovered, err := skills.Discover(storeDir)
	if err != nil {
		return err
	}

	selected := selectSkills(discovered, args)
	if len(selected) == 0 {
		return fmt.Errorf("no matching skills: %s", strings.Join(args, ", "))
	}

	skillMDPath := filepath.Join(selected[0].Path, "SKILL.md")
	editorCmd := exec.Command("sh", "-c", "$EDITOR \"$1\"", "bond-edit", skillMDPath)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = cmd.OutOrStdout()
	editorCmd.Stderr = cmd.ErrOrStderr()

	if err := editorCmd.Run(); err != nil {
		return fmt.Errorf("failed to run editor: %w", err)
	}

	return nil
}
