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

func addLinkedSkill(_ *cobra.Command, invocation Invocation, storedSkillPath string) error {
	storedPath, err := skill.ParseStoredPath(storedSkillPath)
	if err != nil {
		return err
	}

	source, err := selectedStoredSkill(invocation.Environment, storedSkillPath, storedPath)
	if err != nil {
		return err
	}
	validationErrors := skill.Validate(source)
	if len(validationErrors) > 0 {
		diagnostics := make([]string, 0, len(validationErrors))
		for _, validationError := range validationErrors {
			diagnostics = append(diagnostics, fmt.Sprintf("%s: %v", storedSkillPath, validationError))
		}

		return fmt.Errorf("%s", strings.Join(diagnostics, "\n"))
	}

	agentsDirectory := filepath.Join(invocation.WorkingDirectory, ".agents")
	agentsExists, err := realDirectoryIfPresent(agentsDirectory, ".agents")
	if err != nil {
		return err
	}

	manifest := emptyProjectManifest()
	if agentsExists {
		manifest, err = readProjectManifest(agentsDirectory)
		if err != nil {
			return err
		}
	}
	for _, record := range manifest.Skills {
		if record.Name == storedPath.Name {
			return fmt.Errorf("ownership for Project Skill %q already exists", storedPath.Name)
		}
	}

	skillsDirectory := filepath.Join(agentsDirectory, "skills")
	skillsExists := false
	if agentsExists {
		skillsExists, err = realDirectoryIfPresent(skillsDirectory, ".agents/skills")
		if err != nil {
			return err
		}
	}
	destination := filepath.Join(skillsDirectory, storedPath.Name)
	if skillsExists {
		if _, err := os.Lstat(destination); err == nil {
			return fmt.Errorf("destination for Project Skill %q already exists", storedPath.Name)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect destination for Project Skill %q: %w", storedPath.Name, err)
		}
	}

	createdAgents, createdSkills, err := createProjectSkillInfrastructure(agentsDirectory, skillsDirectory, agentsExists, skillsExists)
	if err != nil {
		return err
	}

	if err := os.Symlink(source, destination); err != nil {
		cause := fmt.Errorf("create linked Project Skill %q: %w; retry with --copy", storedPath.Name, err)

		return errors.Join(cause, rollbackAddedSkill("", skillsDirectory, agentsDirectory, false, createdSkills, createdAgents))
	}

	manifest.Skills = append(manifest.Skills, managedSkillRecord{
		Name:        storedPath.Name,
		Source:      storedSkillPath,
		Mode:        linkMode,
		Destination: filepath.ToSlash(filepath.Join(".agents", "skills", storedPath.Name)),
	})
	if err := writeProjectManifest(agentsDirectory, manifest); err != nil {
		return errors.Join(err, rollbackAddedSkill(destination, skillsDirectory, agentsDirectory, true, createdSkills, createdAgents))
	}

	return nil
}

func selectedStoredSkill(environment []string, storedSkillPath string, storedPath skill.StoredPath) (string, error) {
	store, err := storePath(environment)
	if err != nil {
		return "", err
	}
	store, err = filepath.Abs(store)
	if err != nil {
		return "", fmt.Errorf("resolve Store: %w", err)
	}

	parent := store
	if storedPath.Organization != "" {
		parent = filepath.Join(store, storedPath.Organization)
		if err := requireRealDirectory(parent, fmt.Sprintf("selected Organization %q", storedPath.Organization)); err != nil {
			return "", err
		}
		if _, err := os.Lstat(filepath.Join(parent, "SKILL.md")); err == nil {
			return "", fmt.Errorf("entry for Organization %q is an existing Stored Skill", storedPath.Organization)
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("inspect Organization %q: %w", storedPath.Organization, err)
		}
	}

	source := filepath.Join(parent, storedPath.Name)
	if err := requireRealDirectory(source, fmt.Sprintf("selected Stored Skill %q", storedSkillPath)); err != nil {
		return "", err
	}

	return source, nil
}

func requireRealDirectory(path, description string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("%s does not exist", description)
	}
	if err != nil {
		return fmt.Errorf("inspect %s: %w", description, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s must not be a symlink", description)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s must be a directory", description)
	}

	return nil
}

func createProjectSkillInfrastructure(agentsDirectory, skillsDirectory string, agentsExists, skillsExists bool) (bool, bool, error) {
	createdAgents := false
	if !agentsExists {
		if err := os.Mkdir(agentsDirectory, 0o755); err != nil {
			return false, false, fmt.Errorf("create Project infrastructure .agents: %w", err)
		}
		createdAgents = true
	}
	if skillsExists {
		return createdAgents, false, nil
	}
	if err := os.Mkdir(skillsDirectory, 0o755); err != nil {
		cause := fmt.Errorf("create Project infrastructure .agents/skills: %w", err)
		if createdAgents {
			if rollbackError := os.Remove(agentsDirectory); rollbackError != nil {
				cause = errors.Join(cause, fmt.Errorf("remove newly created Project infrastructure .agents: %w", rollbackError))
			}
		}

		return false, false, cause
	}

	return createdAgents, true, nil
}

func rollbackAddedSkill(destination, skillsDirectory, agentsDirectory string, destinationCreated, skillsCreated, agentsCreated bool) error {
	var rollbackErrors []error
	if destinationCreated {
		if err := os.Remove(destination); err != nil {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("remove newly created Project Skill: %w", err))
		}
	}
	if skillsCreated {
		if err := os.Remove(skillsDirectory); err != nil {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("remove newly created Project infrastructure .agents/skills: %w", err))
		}
	}
	if agentsCreated {
		if err := os.Remove(agentsDirectory); err != nil {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("remove newly created Project infrastructure .agents: %w", err))
		}
	}

	return errors.Join(rollbackErrors...)
}
