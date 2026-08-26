package application

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/edsonjaramillo/bond/internal/skill"
	"github.com/kballard/go-shellquote"
	"github.com/spf13/cobra"
)

func newSkillDraft(command *cobra.Command, invocation Invocation, storedSkillPath string) error {
	storedPath, err := skill.ParseStoredPath(storedSkillPath)
	if err != nil {
		return err
	}

	store, err := storePath(invocation.Environment)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(store, 0o755); err != nil {
		return fmt.Errorf("create Store for %q: %w", storedSkillPath, err)
	}

	parent := store
	if storedPath.Organization != "" {
		parent = filepath.Join(store, storedPath.Organization)
		if err := ensureOrganization(parent, storedPath.Organization); err != nil {
			return err
		}
	}

	skillDraftDirectory := filepath.Join(parent, storedPath.Name)
	if _, err := os.Lstat(skillDraftDirectory); err == nil {
		return fmt.Errorf("destination for Stored Skill %q already exists", storedSkillPath)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect Stored Skill destination %q: %w", storedSkillPath, err)
	}
	if err := os.Mkdir(skillDraftDirectory, 0o755); err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("destination for Stored Skill %q already exists", storedSkillPath)
		}

		return fmt.Errorf("create Skill Draft %q: %w", storedSkillPath, err)
	}

	document := fmt.Sprintf("---\nname: %s\ndescription: \"\"\n---\n", storedPath.Name)
	documentPath := filepath.Join(skillDraftDirectory, "SKILL.md")
	if err := os.WriteFile(documentPath, []byte(document), 0o644); err != nil {
		_ = os.Remove(skillDraftDirectory)

		return fmt.Errorf("create Skill Draft %q: %w", storedSkillPath, err)
	}

	editor, configured := environmentValue(invocation.Environment, "EDITOR")
	if !configured || editor == "" {
		return nil
	}
	if err := runEditor(command, invocation.Environment, skillDraftDirectory, editor); err != nil {
		return err
	}

	validationErrors := skill.Validate(skillDraftDirectory)
	if len(validationErrors) == 0 {
		return nil
	}

	diagnostics := make([]string, 0, len(validationErrors))
	for _, validationError := range validationErrors {
		diagnostics = append(diagnostics, fmt.Sprintf("%s: %v", storedSkillPath, validationError))
	}

	return fmt.Errorf("%s", strings.Join(diagnostics, "\n"))
}

func ensureOrganization(directory, name string) error {
	info, err := os.Lstat(directory)
	if os.IsNotExist(err) {
		if err := os.Mkdir(directory, 0o755); err != nil {
			return fmt.Errorf("create Organization %q: %w", name, err)
		}

		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect Organization %q: %w", name, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("entry for Organization %q is not a directory", name)
	}
	if _, err := os.Lstat(filepath.Join(directory, "SKILL.md")); err == nil {
		return fmt.Errorf("entry for Organization %q is an existing Stored Skill", name)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect Organization %q: %w", name, err)
	}

	return nil
}

func runEditor(command *cobra.Command, environment []string, directory, editorSpec string) error {
	arguments, err := shellquote.Split(editorSpec)
	if err != nil {
		return fmt.Errorf("parse EDITOR: %w", err)
	}
	if len(arguments) == 0 {
		return fmt.Errorf("parse EDITOR: no executable")
	}

	editor := exec.CommandContext(command.Context(), arguments[0], append(arguments[1:], "SKILL.md")...)
	editor.Dir = directory
	editor.Env = environment
	editor.Stdin = command.InOrStdin()
	editor.Stdout = command.OutOrStdout()
	editor.Stderr = command.ErrOrStderr()
	if err := editor.Run(); err != nil {
		return fmt.Errorf("run editor: %w", err)
	}

	return nil
}
