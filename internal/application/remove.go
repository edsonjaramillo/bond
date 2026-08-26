package application

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/edsonjaramillo/bond/internal/skill"
	"github.com/spf13/cobra"
)

const (
	afterFirstRemoval = "after-first-removal"
	afterAllRemovals  = "after-all-removals"
)

type requestedRemoval struct {
	argument string
	record   managedSkillRecord
	identity filesystemIdentity
	present  bool
	errors   []string
}

func removeSkills(command *cobra.Command, invocation Invocation, names []string) (resultError error) {
	lock, err := acquireProjectLock(command.Context(), invocation.WorkingDirectory, invocation.projectLockTimeout)
	if err != nil {
		return err
	}
	defer func() {
		resultError = errors.Join(resultError, lock.release())
	}()

	if _, _, _, err := preflightRemovals(invocation.WorkingDirectory, names); err != nil {
		return errors.Join(err, recoverCommittedRemove(invocation.WorkingDirectory))
	}

	agentsDirectory := filepath.Join(invocation.WorkingDirectory, ".agents")
	if err := recoverInterruptedAdd(agentsDirectory); err != nil {
		return err
	}
	requests, manifest, nextManifest, err := preflightRemovals(invocation.WorkingDirectory, names)
	if err != nil {
		return err
	}

	return removePreflightedSkills(invocation, requests, manifest, nextManifest)
}

func removePreflightedSkills(invocation Invocation, requests []requestedRemoval, manifest, nextManifest projectManifest) error {
	agentsDirectory := filepath.Join(invocation.WorkingDirectory, ".agents")
	stageDirectory, err := os.MkdirTemp(agentsDirectory, ".bond-stage-")
	if err != nil {
		return fmt.Errorf("create Project transaction staging: %w", err)
	}
	journal := addTransactionJournal{
		Version:                 1,
		Operation:               "remove",
		StageDirectory:          filepath.Base(stageDirectory),
		PreviousManifestExisted: true,
		PreviousManifest:        manifest,
		NextManifest:            nextManifest,
		Removals:                make([]stagedRemoval, 0, len(requests)),
	}
	for _, request := range requests {
		journal.Removals = append(journal.Removals, stagedRemoval{
			Name:        request.record.Name,
			Mode:        request.record.Mode,
			Identity:    request.identity,
			Destination: filepath.ToSlash(filepath.Join("skills", request.record.Name)),
			Present:     request.present,
		})
	}
	if err := syncDirectory(stageDirectory); err != nil {
		return errors.Join(err, removeUnusedStage(agentsDirectory, stageDirectory))
	}
	if err := writeAddJournal(agentsDirectory, journal); err != nil {
		if _, journalError := os.Lstat(journalPath(agentsDirectory)); journalError == nil {
			return errors.Join(err, rollbackRemoveTransaction(agentsDirectory, journal, false))
		}

		return errors.Join(err, removeUnusedStage(agentsDirectory, stageDirectory))
	}
	if invocation.transactionInterruptionPoint == afterJournalWrite {
		return fmt.Errorf("interrupted Project transaction after journal write")
	}

	stagedCount := 0
	for _, removal := range journal.Removals {
		if !removal.Present {
			continue
		}
		destination := filepath.Join(agentsDirectory, filepath.FromSlash(removal.Destination))
		staged := filepath.Join(stageDirectory, removal.Name)
		if err := os.Rename(destination, staged); err != nil {
			return errors.Join(fmt.Errorf("stage removal of Project Skill %q: %w", removal.Name, err), rollbackRemoveTransaction(agentsDirectory, journal, false))
		}
		if err := syncDirectory(filepath.Dir(destination)); err != nil {
			return errors.Join(err, rollbackRemoveTransaction(agentsDirectory, journal, false))
		}
		if err := syncDirectory(stageDirectory); err != nil {
			return errors.Join(err, rollbackRemoveTransaction(agentsDirectory, journal, false))
		}
		stagedCount++
		if stagedCount == 1 && invocation.transactionInterruptionPoint == afterFirstRemoval {
			return fmt.Errorf("interrupted Project transaction after first removal")
		}
		if stagedCount == 1 && invocation.transactionFailurePoint == afterFirstRemoval {
			return errors.Join(fmt.Errorf("remove Project Skill batch: injected failure"), rollbackRemoveTransaction(agentsDirectory, journal, false))
		}
	}
	if invocation.transactionInterruptionPoint == afterAllRemovals {
		return fmt.Errorf("interrupted Project transaction after all removals")
	}
	if err := writeProjectManifest(agentsDirectory, nextManifest); err != nil {
		return errors.Join(err, rollbackRemoveTransaction(agentsDirectory, journal, true))
	}
	if invocation.transactionInterruptionPoint == afterManifestWrite {
		return fmt.Errorf("interrupted Project transaction after manifest write")
	}
	if invocation.transactionInterruptionPoint == afterStageRemoval {
		if err := removeCommittedRemovalStage(agentsDirectory, journal); err != nil {
			return err
		}

		return fmt.Errorf("interrupted Project transaction after staging removal")
	}
	if err := cleanupCommittedRemove(agentsDirectory, journal); err != nil {
		return err
	}

	return nil
}

func preflightRemovals(project string, arguments []string) ([]requestedRemoval, projectManifest, projectManifest, error) {
	requests := make([]requestedRemoval, len(arguments))
	seen := make(map[string]bool)
	for index, argument := range arguments {
		request := requestedRemoval{argument: argument}
		parsed, err := skill.ParseStoredPath(argument)
		if err != nil || parsed.Organization != "" {
			request.errors = append(request.errors, "Skill Name must use lowercase kebab-case")
		} else if seen[argument] {
			request.errors = append(request.errors, "Skill Name is repeated")
		}
		seen[argument] = true
		requests[index] = request
	}

	agentsDirectory := filepath.Join(project, ".agents")
	agentsExists, agentsError := realDirectoryIfPresent(agentsDirectory, ".agents")
	manifest := emptyProjectManifest()
	var manifestError error
	if agentsError == nil && agentsExists {
		manifest, _, manifestError = readManifestState(agentsDirectory)
	}
	skillsExists := false
	var skillsError error
	if agentsError == nil && agentsExists {
		skillsExists, skillsError = realDirectoryIfPresent(filepath.Join(agentsDirectory, "skills"), ".agents/skills")
	}

	records := make(map[string]managedSkillRecord, len(manifest.Skills))
	for _, record := range manifest.Skills {
		records[record.Name] = record
	}
	if agentsError == nil && manifestError == nil {
		for index := range requests {
			request := &requests[index]
			if len(request.errors) > 0 && (request.argument == "" || records[request.argument].Name == "") {
				continue
			}
			record, managed := records[request.argument]
			if !managed {
				request.errors = append(request.errors, "no Managed Skill with this Skill Name exists")
				continue
			}
			request.record = record
			if skillsError != nil || !skillsExists {
				continue
			}
			destination := filepath.Join(project, filepath.FromSlash(record.Destination))
			identity, err := identifyPath(destination)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				request.errors = append(request.errors, fmt.Sprintf("inspect recorded destination: %v", err))
				continue
			}
			request.identity = identity
			request.present = true
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
		return nil, projectManifest{}, projectManifest{}, fmt.Errorf("%s", strings.Join(diagnostics, "\n"))
	}

	removed := make(map[string]bool, len(requests))
	for _, request := range requests {
		removed[request.argument] = true
	}
	nextManifest := emptyProjectManifest()
	for _, record := range manifest.Skills {
		if !removed[record.Name] {
			nextManifest.Skills = append(nextManifest.Skills, record)
		}
	}

	return requests, manifest, nextManifest, nil
}

func recoverCommittedRemove(project string) error {
	agentsDirectory := filepath.Join(project, ".agents")
	exists, err := realDirectoryIfPresent(agentsDirectory, ".agents")
	if err != nil || !exists {
		return nil
	}
	journal, journalExists, err := readAddJournal(agentsDirectory)
	if err != nil || !journalExists || journal.Operation != "remove" {
		return nil
	}
	current, currentExists, err := readManifestState(agentsDirectory)
	if err != nil {
		return nil
	}
	if currentExists && reflect.DeepEqual(current, journal.NextManifest) {
		return cleanupCommittedRemove(agentsDirectory, journal)
	}

	return nil
}

func recoverInterruptedRemove(agentsDirectory string, journal addTransactionJournal, current projectManifest, currentExists bool) error {
	if currentExists && reflect.DeepEqual(current, journal.NextManifest) {
		return cleanupCommittedRemove(agentsDirectory, journal)
	}
	if !currentExists || !reflect.DeepEqual(current, journal.PreviousManifest) {
		return manualRecoveryError(agentsDirectory, "manifest no longer matches the transaction")
	}

	return rollbackRemoveTransaction(agentsDirectory, journal, false)
}

func rollbackRemoveTransaction(agentsDirectory string, journal addTransactionJournal, restoreManifest bool) error {
	if err := validateRemovalRollback(agentsDirectory, journal); err != nil {
		return err
	}
	if restoreManifest {
		if err := writeProjectManifest(agentsDirectory, journal.PreviousManifest); err != nil {
			return err
		}
	}
	stageDirectory := filepath.Join(agentsDirectory, journal.StageDirectory)
	for _, removal := range journal.Removals {
		if !removal.Present {
			continue
		}
		staged := filepath.Join(stageDirectory, removal.Name)
		destination := filepath.Join(agentsDirectory, filepath.FromSlash(removal.Destination))
		if pathHasIdentity(staged, removal.Identity) {
			if err := renameNoReplace(staged, destination); err != nil {
				if errors.Is(err, os.ErrExist) {
					return manualRecoveryError(agentsDirectory, fmt.Sprintf("destination for Project Skill %q was created during recovery", removal.Name))
				}

				return fmt.Errorf("restore Project Skill %q: %w", removal.Name, err)
			}
			if err := syncDirectory(filepath.Dir(destination)); err != nil {
				return err
			}
		}
	}
	if err := removeEmptyStageDirectory(agentsDirectory, journal); err != nil {
		return err
	}

	return removeJournalDurably(agentsDirectory, journal)
}

func validateRemovalRollback(agentsDirectory string, journal addTransactionJournal) error {
	if err := validateRemovalStageEntries(agentsDirectory, journal); err != nil {
		return err
	}
	stageDirectory := filepath.Join(agentsDirectory, journal.StageDirectory)
	for _, removal := range journal.Removals {
		if !removal.Present {
			continue
		}
		staged := filepath.Join(stageDirectory, removal.Name)
		destination := filepath.Join(agentsDirectory, filepath.FromSlash(removal.Destination))
		stagedMatches := pathHasIdentity(staged, removal.Identity)
		destinationMatches := pathHasIdentity(destination, removal.Identity)
		if (stagedMatches && !pathExists(destination)) || (!pathExists(staged) && destinationMatches) {
			continue
		}
		return manualRecoveryError(agentsDirectory, fmt.Sprintf("Project Skill %q conflicts with interrupted removal", removal.Name))
	}

	return nil
}

func cleanupCommittedRemove(agentsDirectory string, journal addTransactionJournal) error {
	if err := ensureProjectSkillDirectory(agentsDirectory); err != nil {
		return err
	}
	if err := removeCommittedRemovalStage(agentsDirectory, journal); err != nil {
		return err
	}

	return removeJournalDurably(agentsDirectory, journal)
}

func ensureProjectSkillDirectory(agentsDirectory string) error {
	skillsDirectory := filepath.Join(agentsDirectory, "skills")
	exists, err := realDirectoryIfPresent(skillsDirectory, ".agents/skills")
	if err != nil || exists {
		return err
	}
	if err := os.Mkdir(skillsDirectory, 0o755); err != nil {
		return fmt.Errorf("create Project infrastructure .agents/skills: %w", err)
	}

	return syncDirectory(agentsDirectory)
}

func removeCommittedRemovalStage(agentsDirectory string, journal addTransactionJournal) error {
	if err := validateRemovalStageEntries(agentsDirectory, journal); err != nil {
		return err
	}
	stageDirectory := filepath.Join(agentsDirectory, journal.StageDirectory)
	for _, removal := range journal.Removals {
		staged := filepath.Join(stageDirectory, removal.Name)
		if !pathExists(staged) {
			continue
		}
		if !pathHasIdentity(staged, removal.Identity) {
			return manualRecoveryError(agentsDirectory, fmt.Sprintf("staged Project Skill %q conflicts with the transaction", removal.Name))
		}
		if err := os.RemoveAll(staged); err != nil {
			return fmt.Errorf("remove staged Project Skill %q: %w", removal.Name, err)
		}
		if err := syncDirectory(stageDirectory); err != nil {
			return err
		}
	}

	return removeEmptyStageDirectory(agentsDirectory, journal)
}

func validateRemovalStageEntries(agentsDirectory string, journal addTransactionJournal) error {
	stageDirectory := filepath.Join(agentsDirectory, journal.StageDirectory)
	entries, err := os.ReadDir(stageDirectory)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read Project transaction staging: %w", err)
	}
	removals := make(map[string]stagedRemoval, len(journal.Removals))
	for _, removal := range journal.Removals {
		removals[removal.Name] = removal
	}
	for _, entry := range entries {
		removal, exists := removals[entry.Name()]
		if !exists || !removal.Present || !pathHasIdentity(filepath.Join(stageDirectory, entry.Name()), removal.Identity) {
			return manualRecoveryError(agentsDirectory, fmt.Sprintf("staging path %q conflicts with the transaction", entry.Name()))
		}
	}

	return nil
}

func pathHasIdentity(path string, want filesystemIdentity) bool {
	got, err := identifyPath(path)

	return err == nil && got == want
}

func pathExists(path string) bool {
	_, err := os.Lstat(path)

	return err == nil
}

func removeUnusedStage(agentsDirectory, stageDirectory string) error {
	if err := os.Remove(stageDirectory); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove Project transaction staging: %w", err)
	}

	return syncDirectory(agentsDirectory)
}
