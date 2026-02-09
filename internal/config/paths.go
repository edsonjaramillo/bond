package config

import (
	"os"
	"path/filepath"
)

// ProjectRoot returns the current working directory for command execution.
func ProjectRoot() (string, error) {
	return os.Getwd()
}

// ProjectAgentsDir returns the project-local .agents directory path.
func ProjectAgentsDir() (string, error) {
	root, err := ProjectRoot()
	if err != nil {
		return "", err
	}
	return ProjectAgentsDirFrom(root), nil
}

// ProjectSkillsDir returns the project-local .agents/skills directory path.
func ProjectSkillsDir() (string, error) {
	root, err := ProjectRoot()
	if err != nil {
		return "", err
	}
	return ProjectSkillsDirFrom(root), nil
}

// ProjectAgentsDirFrom builds the .agents path from an explicit project root.
func ProjectAgentsDirFrom(root string) string {
	return filepath.Join(root, ".agents")
}

// ProjectSkillsDirFrom builds the .agents/skills path from an explicit project root.
func ProjectSkillsDirFrom(root string) string {
	return filepath.Join(ProjectAgentsDirFrom(root), "skills")
}

// GlobalSkillsDir returns the global Bond skills directory based on XDG conventions.
func GlobalSkillsDir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "bond"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".config", "bond"), nil
}
