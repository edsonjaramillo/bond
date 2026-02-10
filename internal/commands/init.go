package commands

import (
	"fmt"
	"os"

	"bond/internal/config"
	"github.com/spf13/cobra"
)

// newInitCmd builds the init command for bootstrapping project directories.
func newInitCmd() *cobra.Command {
	var global bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize .agents/skills in the current project",
		RunE: func(cmd *cobra.Command, args []string) error {
			if global {
				globalDir, err := config.GlobalSkillsDir()
				if err != nil {
					return err
				}

				created, err := ensureDir(globalDir)
				if err != nil {
					return err
				}

				level := levelInfo
				message := "global bond directory already exists"
				if created {
					level = levelOK
					message = "initialized global bond directory"
				}

				return printOut(cmd, level, message)
			}

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

			createdCount := 0
			if agentsCreated {
				createdCount++
			}
			if skillsCreated {
				createdCount++
			}

			level := levelInfo
			message := ".agents/skills already exists"
			if createdCount > 0 {
				level = levelOK
				message = "initialized .agents/skills"
			}

			return printOut(cmd, level, message)
		},
	}

	cmd.Flags().BoolVar(&global, "global", false, "Initialize global Bond directory (XDG_CONFIG_HOME/bond or ~/.config/bond)")
	return cmd
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
