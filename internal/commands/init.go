package commands

import (
	"fmt"
	"os"

	"bond/internal/config"
	"github.com/spf13/cobra"
)

// newInitCmd builds the init command for bootstrapping project directories.
func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize .agents/skills in the current project",
		RunE: func(cmd *cobra.Command, args []string) error {
			agentsDir, err := config.ProjectAgentsDir()
			if err != nil {
				return err
			}
			skillsDir, err := config.ProjectSkillsDir()
			if err != nil {
				return err
			}

			agentsCreated, err := ensureDir(agentsDir)
			if err != nil {
				return err
			}
			skillsCreated, err := ensureDir(skillsDir)
			if err != nil {
				return err
			}

			if err := printInitStatus(cmd, ".agents", agentsCreated); err != nil {
				return err
			}
			if err := printInitStatus(cmd, ".agents/skills", skillsCreated); err != nil {
				return err
			}
			return nil
		},
	}
}

// ensureDir creates path when missing and reports whether it was created.
func ensureDir(path string) (bool, error) {
	if info, err := os.Stat(path); err == nil {
		if info.IsDir() {
			return false, nil
		}
		return false, fmt.Errorf("%q exists and is not a directory", path)
	} else if !os.IsNotExist(err) {
		return false, err
	}

	if err := os.MkdirAll(path, 0o755); err != nil {
		return false, err
	}
	return true, nil
}

// printInitStatus prints a consistent status line for init directory checks.
func printInitStatus(cmd *cobra.Command, label string, created bool) error {
	if created {
		return printOut(cmd, levelOK, "created %s", label)
	}
	return printOut(cmd, levelInfo, "already exists %s", label)
}
