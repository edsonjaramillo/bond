package application

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/edsonjaramillo/bond/internal/skill"
	"github.com/spf13/cobra"
)

func listProjectSkills(command *cobra.Command, invocation Invocation) error {
	collection, exists, err := projectSkillCollection(invocation.WorkingDirectory)
	if err != nil {
		return err
	}
	if !exists {
		return printNoSkillsFound(command.OutOrStdout())
	}

	discovery, err := skill.DiscoverProject(collection)
	if err != nil {
		return fmt.Errorf("read Project Skills: %w", err)
	}
	for _, name := range discovery.Names {
		if _, err := fmt.Fprintln(command.OutOrStdout(), name); err != nil {
			return err
		}
	}
	if len(discovery.Diagnostics) > 0 {
		return fmt.Errorf("%s", strings.Join(discovery.Diagnostics, "\n"))
	}
	if len(discovery.Names) == 0 {
		return printNoSkillsFound(command.OutOrStdout())
	}

	return nil
}

func editProjectSkill(command *cobra.Command, invocation Invocation, name string) error {
	if !validProjectSkillBasename(name) {
		return fmt.Errorf("argument for Project Skill %q must be exactly one directory basename", name)
	}

	collection, exists, err := projectSkillCollection(invocation.WorkingDirectory)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("no Project Skill named %q exists", name)
	}

	directory := filepath.Join(collection, name)
	if _, err := skill.ResolveProjectSkillDirectory(directory); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("no Project Skill named %q exists", name)
		}

		return fmt.Errorf("invalid Project Skill %q: %w", name, err)
	}

	editor, configured := environmentValue(invocation.Environment, "EDITOR")
	if !configured || strings.TrimSpace(editor) == "" {
		return fmt.Errorf("EDITOR is not set")
	}
	if err := runEditor(command, invocation.Environment, directory, editor); err != nil {
		return err
	}

	validationErrors := skill.ValidateProjectSkill(directory, name)
	if len(validationErrors) == 0 {
		return nil
	}
	diagnostics := make([]string, 0, len(validationErrors))
	for _, validationError := range validationErrors {
		diagnostics = append(diagnostics, fmt.Sprintf("%s: %v", name, validationError))
	}

	return fmt.Errorf("%s", strings.Join(diagnostics, "\n"))
}

func projectSkillCollection(project string) (string, bool, error) {
	agents := filepath.Join(project, ".agents")
	exists, err := realDirectoryIfPresent(agents, ".agents")
	if err != nil || !exists {
		return "", false, err
	}

	collection := filepath.Join(agents, "skills")
	exists, err = realDirectoryIfPresent(collection, ".agents/skills")
	if err != nil || !exists {
		return "", false, err
	}

	return collection, true, nil
}

func realDirectoryIfPresent(path, projectPath string) (bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect Project infrastructure %s: %w", projectPath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("path for Project infrastructure %s must not be a symlink", projectPath)
	}
	if !info.IsDir() {
		return false, fmt.Errorf("path for Project infrastructure %s must be a directory", projectPath)
	}

	return true, nil
}

func validProjectSkillBasename(name string) bool {
	return name != "" && name != "." && name != ".." && filepath.Base(name) == name
}
