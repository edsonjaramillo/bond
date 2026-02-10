package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"bond/internal/config"
	"github.com/spf13/cobra"
)

const defaultCreateDescription = "TODO: describe this skill"

var createSkillNamePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// newCreateCmd builds the command that scaffolds a new store skill directory.
func newCreateCmd() *cobra.Command {
	var description string

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new skill scaffold in the store directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreate(cmd, args[0], description, cmd.Flags().Changed("description"))
		},
	}

	cmd.Flags().StringVar(&description, "description", defaultCreateDescription, "Initial skill description")
	return cmd
}

// runCreate creates one new skill directory in the store with a starter SKILL.md.
func runCreate(cmd *cobra.Command, name, description string, descriptionProvided bool) error {
	if err := validateCreateSkillName(name); err != nil {
		return err
	}

	storeDir, err := config.StoreSkillsDir()
	if err != nil {
		return err
	}
	if _, err := ensureDir(storeDir); err != nil {
		return err
	}

	skillDir := filepath.Join(storeDir, name)
	if _, err := os.Stat(skillDir); err == nil {
		return fmt.Errorf("skill %q already exists in store directory %q", name, storeDir)
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		return err
	}

	needsDescriptionWarning := !descriptionProvided || strings.TrimSpace(description) == ""
	if strings.TrimSpace(description) == "" {
		description = defaultCreateDescription
	}

	skillFile := filepath.Join(skillDir, "SKILL.md")
	contents := fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n", name, strconv.Quote(description))
	if err := os.WriteFile(skillFile, []byte(contents), 0o644); err != nil {
		return err
	}

	if err := printOut(cmd, levelOK, "created %s", name); err != nil {
		return err
	}
	if needsDescriptionWarning {
		return printOut(cmd, levelWarn, "add a description that describes the skill")
	}
	return nil
}

func validateCreateSkillName(name string) error {
	nameLen := utf8.RuneCountInString(name)
	if nameLen == 0 {
		return fmt.Errorf("skill name must not be empty")
	}
	if nameLen > 64 {
		return fmt.Errorf("skill name %q is %d characters; maximum is 64", name, nameLen)
	}
	if !createSkillNamePattern.MatchString(name) {
		return fmt.Errorf("skill name %q must use lowercase letters, numbers, and single hyphens only", name)
	}
	return nil
}
