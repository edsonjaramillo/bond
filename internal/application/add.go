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

const (
	afterJournalWrite  = "after-journal-write"
	afterFirstPublish  = "after-first-publish"
	afterAllPublishes  = "after-all-publishes"
	afterManifestWrite = "after-manifest-write"
	afterStageRemoval  = "after-stage-removal"
)

type requestedInstallation struct {
	argument string
	path     skill.StoredPath
	source   string
	errors   []string
}

func addLinkedSkills(command *cobra.Command, invocation Invocation, storedSkillPaths []string) (resultError error) {
	lock, err := acquireProjectLock(command.Context(), invocation.WorkingDirectory, invocation.projectLockTimeout)
	if err != nil {
		return err
	}
	defer func() {
		resultError = errors.Join(resultError, lock.release())
	}()

	agentsDirectory := filepath.Join(invocation.WorkingDirectory, ".agents")
	agentsExists, err := realDirectoryIfPresent(agentsDirectory, ".agents")
	if err != nil {
		return err
	}
	if agentsExists {
		if err := recoverInterruptedAdd(agentsDirectory); err != nil {
			return err
		}
	}

	requests, manifest, manifestExists, agentsExists, skillsExists, err := preflightLinkedSkills(invocation, storedSkillPaths)
	if err != nil {
		return err
	}

	createdAgents, createdSkills, err := createProjectSkillInfrastructure(
		agentsDirectory,
		filepath.Join(agentsDirectory, "skills"),
		agentsExists,
		skillsExists,
	)
	if err != nil {
		return err
	}

	stageDirectory, err := os.MkdirTemp(agentsDirectory, ".bond-stage-")
	if err != nil {
		return errors.Join(
			fmt.Errorf("create Project transaction staging: %w", err),
			rollbackAddedSkill("", filepath.Join(agentsDirectory, "skills"), agentsDirectory, false, createdSkills, createdAgents),
		)
	}

	nextManifest := manifest
	installations := make([]stagedInstallation, 0, len(requests))
	for _, request := range requests {
		staged := filepath.Join(stageDirectory, request.path.Name)
		if err := os.Symlink(request.source, staged); err != nil {
			cause := fmt.Errorf("create linked Project Skill %q: %w; retry with --copy", request.path.Name, err)
			return errors.Join(cause, cleanupStagedAdd(stageDirectory, filepath.Join(agentsDirectory, "skills"), agentsDirectory, createdSkills, createdAgents))
		}
		nextManifest.Skills = append(nextManifest.Skills, managedSkillRecord{
			Name:        request.path.Name,
			Source:      request.argument,
			Mode:        linkMode,
			Destination: filepath.ToSlash(filepath.Join(".agents", "skills", request.path.Name)),
		})
		installations = append(installations, stagedInstallation{
			Name:        request.path.Name,
			Source:      request.source,
			Destination: filepath.ToSlash(filepath.Join("skills", request.path.Name)),
		})
	}

	if err := syncDirectory(stageDirectory); err != nil {
		return errors.Join(err, cleanupStagedAdd(stageDirectory, filepath.Join(agentsDirectory, "skills"), agentsDirectory, createdSkills, createdAgents))
	}

	journal := addTransactionJournal{
		Version:                 1,
		StageDirectory:          filepath.Base(stageDirectory),
		CreatedAgentsDirectory:  createdAgents,
		CreatedSkillsDirectory:  createdSkills,
		PreviousManifestExisted: manifestExists,
		PreviousManifest:        manifest,
		NextManifest:            nextManifest,
		Installations:           installations,
	}
	if err := writeAddJournal(agentsDirectory, journal); err != nil {
		if _, journalError := os.Lstat(journalPath(agentsDirectory)); journalError == nil {
			return errors.Join(err, rollbackAddTransaction(agentsDirectory, journal, false))
		} else if !os.IsNotExist(journalError) {
			return errors.Join(err, fmt.Errorf("inspect Project transaction journal after write failure: %w", journalError))
		}

		return errors.Join(err, cleanupStagedAdd(stageDirectory, filepath.Join(agentsDirectory, "skills"), agentsDirectory, createdSkills, createdAgents))
	}
	if invocation.transactionInterruptionPoint == afterJournalWrite {
		return fmt.Errorf("interrupted Project transaction after journal write")
	}

	for index, installation := range installations {
		staged := filepath.Join(stageDirectory, installation.Name)
		destination := filepath.Join(agentsDirectory, filepath.FromSlash(installation.Destination))
		if err := os.Rename(staged, destination); err != nil {
			cause := fmt.Errorf("publish linked Project Skill %q: %w", installation.Name, err)

			return errors.Join(cause, rollbackAddTransaction(agentsDirectory, journal, false))
		}
		if err := syncDirectory(stageDirectory); err != nil {
			return errors.Join(err, rollbackAddTransaction(agentsDirectory, journal, false))
		}
		if err := syncDirectory(filepath.Dir(destination)); err != nil {
			return errors.Join(err, rollbackAddTransaction(agentsDirectory, journal, false))
		}
		if index == 0 && invocation.transactionInterruptionPoint == afterFirstPublish {
			return fmt.Errorf("interrupted Project transaction after first publication")
		}
		if index == 0 && invocation.transactionFailurePoint == afterFirstPublish {
			return errors.Join(fmt.Errorf("publish linked Project Skill batch: injected failure"), rollbackAddTransaction(agentsDirectory, journal, false))
		}
	}
	if invocation.transactionInterruptionPoint == afterAllPublishes {
		return fmt.Errorf("interrupted Project transaction after all publications")
	}
	if err := writeProjectManifest(agentsDirectory, nextManifest); err != nil {
		return errors.Join(err, rollbackAddTransaction(agentsDirectory, journal, true))
	}
	if invocation.transactionInterruptionPoint == afterManifestWrite {
		return fmt.Errorf("interrupted Project transaction after manifest write")
	}
	if invocation.transactionInterruptionPoint == afterStageRemoval {
		if err := os.RemoveAll(stageDirectory); err != nil {
			return fmt.Errorf("remove Project transaction staging: %w", err)
		}
		if err := syncDirectory(agentsDirectory); err != nil {
			return err
		}

		return fmt.Errorf("interrupted Project transaction after staging removal")
	}
	if err := cleanupCommittedAdd(agentsDirectory, journal); err != nil {
		return errors.Join(err, rollbackAddTransaction(agentsDirectory, journal, true))
	}

	return nil
}

func cleanupStagedAdd(stageDirectory, skillsDirectory, agentsDirectory string, createdSkills, createdAgents bool) error {
	var cleanupError error
	if err := os.RemoveAll(stageDirectory); err != nil {
		cleanupError = fmt.Errorf("remove Project transaction staging: %w", err)
	} else if err := syncDirectory(agentsDirectory); err != nil {
		cleanupError = err
	}

	return errors.Join(cleanupError, rollbackAddedSkill("", skillsDirectory, agentsDirectory, false, createdSkills, createdAgents))
}

func preflightLinkedSkills(invocation Invocation, arguments []string) ([]requestedInstallation, projectManifest, bool, bool, bool, error) {
	requests := make([]requestedInstallation, len(arguments))
	seenArguments := make(map[string]bool)
	seenNames := make(map[string]string)
	for index, argument := range arguments {
		request := requestedInstallation{argument: argument}
		storedPath, err := skill.ParseStoredPath(argument)
		if err != nil {
			request.errors = append(request.errors, err.Error())
			requests[index] = request

			continue
		}
		request.path = storedPath
		if seenArguments[argument] {
			request.errors = append(request.errors, "Stored Skill path is repeated")
		}
		seenArguments[argument] = true
		if previous, exists := seenNames[storedPath.Name]; exists && previous != argument {
			request.errors = append(request.errors, fmt.Sprintf("Skill Name %q is also requested by %q", storedPath.Name, previous))
		} else if !exists {
			seenNames[storedPath.Name] = argument
		}
		source, err := selectedStoredSkill(invocation.Environment, argument, storedPath)
		if err != nil {
			request.errors = append(request.errors, err.Error())
		} else {
			request.source = source
			for _, validationError := range skill.Validate(source) {
				request.errors = append(request.errors, validationError.Error())
			}
		}
		requests[index] = request
	}

	agentsDirectory := filepath.Join(invocation.WorkingDirectory, ".agents")
	agentsExists, agentsError := realDirectoryIfPresent(agentsDirectory, ".agents")
	manifest := emptyProjectManifest()
	manifestExists := false
	var manifestError error
	if agentsError == nil && agentsExists {
		manifest, manifestExists, manifestError = readManifestState(agentsDirectory)
	}

	skillsDirectory := filepath.Join(agentsDirectory, "skills")
	skillsExists := false
	var skillsError error
	if agentsError == nil && agentsExists {
		skillsExists, skillsError = realDirectoryIfPresent(skillsDirectory, ".agents/skills")
	}

	if agentsError == nil && skillsError == nil {
		ownedNames := make(map[string]bool)
		for _, record := range manifest.Skills {
			ownedNames[record.Name] = true
		}
		for index := range requests {
			request := &requests[index]
			if request.path.Name == "" {
				continue
			}
			if ownedNames[request.path.Name] {
				request.errors = append(request.errors, fmt.Sprintf("ownership for Project Skill %q already exists", request.path.Name))
			}
			if skillsExists {
				destination := filepath.Join(skillsDirectory, request.path.Name)
				if _, err := os.Lstat(destination); err == nil {
					request.errors = append(request.errors, fmt.Sprintf("destination for Project Skill %q already exists", request.path.Name))
				} else if !os.IsNotExist(err) {
					request.errors = append(request.errors, fmt.Sprintf("inspect destination for Project Skill %q: %v", request.path.Name, err))
				}
			}
		}
	}

	var diagnostics []string
	for _, request := range requests {
		for _, requestError := range request.errors {
			diagnostics = append(diagnostics, fmt.Sprintf("%s: %s", request.argument, requestError))
		}
	}
	for _, projectError := range []error{agentsError, skillsError, manifestError} {
		if projectError != nil {
			diagnostics = append(diagnostics, projectError.Error())
		}
	}
	if len(diagnostics) > 0 {
		return nil, projectManifest{}, false, false, false, fmt.Errorf("%s", strings.Join(diagnostics, "\n"))
	}

	return requests, manifest, manifestExists, agentsExists, skillsExists, nil
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
		if err := syncDirectory(filepath.Dir(agentsDirectory)); err != nil {
			return false, false, errors.Join(err, rollbackAddedSkill("", skillsDirectory, agentsDirectory, false, false, true))
		}
		createdAgents = true
	}
	if skillsExists {
		return createdAgents, false, nil
	}
	if err := os.Mkdir(skillsDirectory, 0o755); err != nil {
		cause := fmt.Errorf("create Project infrastructure .agents/skills: %w", err)
		if createdAgents {
			cause = errors.Join(cause, rollbackAddedSkill("", skillsDirectory, agentsDirectory, false, false, true))
		}

		return false, false, cause
	}

	if err := syncDirectory(agentsDirectory); err != nil {
		return false, false, errors.Join(err, rollbackAddedSkill("", skillsDirectory, agentsDirectory, false, true, createdAgents))
	}

	return createdAgents, true, nil
}

func rollbackAddedSkill(destination, skillsDirectory, agentsDirectory string, destinationCreated, skillsCreated, agentsCreated bool) error {
	var rollbackErrors []error
	if destinationCreated {
		if err := os.Remove(destination); err != nil {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("remove newly created Project Skill: %w", err))
		} else if err := syncDirectory(filepath.Dir(destination)); err != nil {
			rollbackErrors = append(rollbackErrors, err)
		}
	}
	if skillsCreated {
		if err := os.Remove(skillsDirectory); err != nil {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("remove newly created Project infrastructure .agents/skills: %w", err))
		} else if err := syncDirectory(agentsDirectory); err != nil {
			rollbackErrors = append(rollbackErrors, err)
		}
	}
	if agentsCreated {
		if err := os.Remove(agentsDirectory); err != nil {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("remove newly created Project infrastructure .agents: %w", err))
		} else if err := syncDirectory(filepath.Dir(agentsDirectory)); err != nil {
			rollbackErrors = append(rollbackErrors, err)
		}
	}

	return errors.Join(rollbackErrors...)
}
