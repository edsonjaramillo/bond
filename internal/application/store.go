package application

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/edsonjaramillo/bond/internal/skill"
	"github.com/spf13/cobra"
)

func listStoredSkills(command *cobra.Command, environment []string) error {
	store, err := storePath(environment)
	if err != nil {
		return err
	}

	discovery, err := skill.DiscoverStore(store)
	if os.IsNotExist(err) {
		return printNoSkillsFound(command.OutOrStdout())
	}
	if err != nil {
		return fmt.Errorf("read Store: %w", err)
	}

	for _, path := range discovery.Paths {
		if _, err := fmt.Fprintln(command.OutOrStdout(), path); err != nil {
			return err
		}
	}
	if len(discovery.Diagnostics) > 0 {
		return fmt.Errorf("%s", strings.Join(discovery.Diagnostics, "\n"))
	}
	if len(discovery.Paths) == 0 {
		return printNoSkillsFound(command.OutOrStdout())
	}

	return nil
}

func printNoSkillsFound(writer io.Writer) error {
	_, err := fmt.Fprintln(writer, "No skills found.")

	return err
}

func storePath(environment []string) (string, error) {
	return centralCollectionPath(environment, skillCollection)
}

func environmentValue(environment []string, name string) (string, bool) {
	prefix := name + "="
	for _, entry := range environment {
		if strings.HasPrefix(entry, prefix) {
			return strings.TrimPrefix(entry, prefix), true
		}
	}

	return "", false
}
