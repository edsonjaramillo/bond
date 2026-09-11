package application

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type requestedResourceRemoval struct {
	name   string
	record managedResourceRecord
	leaves []resourceJournalLeaf
	errors []string
}

func removeResources(command *cobra.Command, invocation Invocation, arguments []string) (resultError error) {
	lock, err := acquireProjectLock(command.Context(), invocation.WorkingDirectory, invocation.projectLockTimeout)
	if err != nil {
		return err
	}
	defer func() { resultError = errors.Join(resultError, lock.release()) }()

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
	requests, manifest, nextManifest, err := preflightResourceRemovals(invocation.WorkingDirectory, arguments)
	if err != nil {
		return err
	}
	stageDirectory, err := os.MkdirTemp(agentsDirectory, ".bond-stage-")
	if err != nil {
		return fmt.Errorf("create Project transaction staging: %w", err)
	}
	journal := addTransactionJournal{
		Version: 1, Operation: "resource-remove", StageDirectory: filepath.Base(stageDirectory),
		PreviousManifestExisted: true, PreviousManifest: manifest, NextManifest: nextManifest,
	}
	index := 0
	for _, request := range requests {
		for _, leaf := range request.leaves {
			leaf.Source = filepath.ToSlash(filepath.Join("leaves", fmt.Sprintf("%06d", index)))
			journal.ResourceRemovals = append(journal.ResourceRemovals, leaf)
			index++
		}
	}
	if err := os.Mkdir(filepath.Join(stageDirectory, "leaves"), 0o755); err != nil {
		return errors.Join(err, os.RemoveAll(stageDirectory))
	}
	if err := syncDirectory(filepath.Join(stageDirectory, "leaves")); err != nil {
		return errors.Join(err, os.RemoveAll(stageDirectory))
	}
	if err := syncDirectory(stageDirectory); err != nil {
		return errors.Join(err, os.RemoveAll(stageDirectory))
	}
	if err := writeAddJournal(agentsDirectory, journal); err != nil {
		if _, journalError := os.Lstat(journalPath(agentsDirectory)); journalError == nil {
			return errors.Join(err, rollbackResourceTransaction(agentsDirectory, journal, false))
		}
		return errors.Join(err, os.RemoveAll(stageDirectory))
	}
	if invocation.transactionInterruptionPoint == afterJournalWrite {
		return fmt.Errorf("interrupted Project transaction after journal write")
	}
	removed := 0
	for _, leaf := range journal.ResourceRemovals {
		if !leaf.Present {
			continue
		}
		destination := filepath.Join(invocation.WorkingDirectory, filepath.FromSlash(leaf.Destination))
		staged := filepath.Join(stageDirectory, filepath.FromSlash(leaf.Source))
		if err := os.Rename(destination, staged); err != nil {
			return errors.Join(fmt.Errorf("stage removal of Resource path %q: %w", leaf.Destination, err), rollbackResourceTransaction(agentsDirectory, journal, false))
		}
		if err := syncDirectory(filepath.Dir(destination)); err != nil {
			return errors.Join(err, rollbackResourceTransaction(agentsDirectory, journal, false))
		}
		if err := syncDirectory(filepath.Dir(staged)); err != nil {
			return errors.Join(err, rollbackResourceTransaction(agentsDirectory, journal, false))
		}
		removed++
		if removed == 1 && invocation.transactionFailurePoint == afterFirstRemoval {
			return errors.Join(fmt.Errorf("remove Resource batch: injected failure"), rollbackResourceTransaction(agentsDirectory, journal, false))
		}
		if removed == 1 && invocation.transactionInterruptionPoint == afterFirstRemoval {
			return fmt.Errorf("interrupted Project transaction after first removal")
		}
	}
	if invocation.transactionInterruptionPoint == afterAllRemovals {
		return fmt.Errorf("interrupted Project transaction after all removals")
	}
	if err := writeProjectManifest(agentsDirectory, nextManifest); err != nil {
		return errors.Join(err, rollbackResourceTransaction(agentsDirectory, journal, true))
	}
	if invocation.transactionInterruptionPoint == afterManifestWrite {
		return fmt.Errorf("interrupted Project transaction after manifest write")
	}
	if invocation.transactionInterruptionPoint == afterStageRemoval {
		if err := removeCommittedResourceRemovalStage(agentsDirectory, journal); err != nil {
			return err
		}
		return fmt.Errorf("interrupted Project transaction after staging removal")
	}

	return cleanupCommittedResourceTransaction(agentsDirectory, journal)
}

func preflightResourceRemovals(project string, arguments []string) ([]requestedResourceRemoval, projectManifest, projectManifest, error) {
	requests := make([]requestedResourceRemoval, len(arguments))
	seen := make(map[string]bool)
	for index, argument := range arguments {
		request := requestedResourceRemoval{name: argument}
		if !validResourceName(argument) {
			request.errors = append(request.errors, "resource name must use lowercase kebab-case")
		}
		if seen[argument] {
			request.errors = append(request.errors, "Resource Name is repeated")
		}
		seen[argument] = true
		requests[index] = request
	}
	agentsDirectory := filepath.Join(project, ".agents")
	agentsExists, agentsError := realDirectoryIfPresent(agentsDirectory, ".agents")
	manifest := emptyProjectManifest()
	manifestExists := false
	var manifestError error
	if agentsError == nil && agentsExists {
		manifest, manifestExists, manifestError = readManifestState(agentsDirectory)
	}
	transactionIdentity, transactionIdentityError := identifyPath(agentsDirectory)
	records := make(map[string]managedResourceRecord)
	for _, record := range manifest.Resources {
		records[record.Name] = record
	}
	if agentsError == nil && manifestError == nil {
		for index := range requests {
			request := &requests[index]
			record, exists := records[request.name]
			if !exists {
				request.errors = append(request.errors, "no Managed Resource with this Resource Name exists")
				continue
			}
			request.record = record
			for _, relative := range record.Paths {
				leaf := resourceJournalLeaf{Resource: record.Name, Destination: relative}
				if err := inspectOwnedResourceAncestors(project, relative); err != nil {
					request.errors = append(request.errors, err.Error())
					continue
				}
				path := filepath.Join(project, filepath.FromSlash(relative))
				info, err := os.Lstat(path)
				if os.IsNotExist(err) {
					request.leaves = append(request.leaves, leaf)
					continue
				}
				if err != nil {
					request.errors = append(request.errors, fmt.Sprintf("inspect owned Resource path %q: %v", relative, err))
					continue
				}
				if info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
					request.errors = append(request.errors, fmt.Sprintf("owned Resource path %q is a directory; refusing recursive removal", relative))
					continue
				}
				identity, err := identifyPath(path)
				if err != nil {
					request.errors = append(request.errors, fmt.Sprintf("identify owned Resource path %q: %v", relative, err))
					continue
				}
				if transactionIdentityError == nil && identity.Device != transactionIdentity.Device {
					request.errors = append(request.errors, fmt.Sprintf("owned Resource path %q is on a different filesystem from the Project transaction area", relative))
					continue
				}
				leaf.Identity = identity
				leaf.Present = true
				request.leaves = append(request.leaves, leaf)
			}
		}
	}
	var diagnostics []string
	for _, request := range requests {
		for _, requestError := range request.errors {
			diagnostics = append(diagnostics, fmt.Sprintf("%s: %s", request.name, requestError))
		}
	}
	for _, projectError := range []error{agentsError, manifestError, transactionIdentityError} {
		if projectError != nil {
			diagnostics = append(diagnostics, projectError.Error())
		}
	}
	if !manifestExists && len(diagnostics) == 0 {
		for _, argument := range arguments {
			diagnostics = append(diagnostics, fmt.Sprintf("%s: no Managed Resource with this Resource Name exists", argument))
		}
	}
	if len(diagnostics) > 0 {
		return nil, projectManifest{}, projectManifest{}, fmt.Errorf("%s", strings.Join(diagnostics, "\n"))
	}
	removed := make(map[string]bool)
	for _, argument := range arguments {
		removed[argument] = true
	}
	nextManifest := manifest
	nextManifest.Resources = make([]managedResourceRecord, 0, len(manifest.Resources)-len(arguments))
	for _, record := range manifest.Resources {
		if !removed[record.Name] {
			nextManifest.Resources = append(nextManifest.Resources, record)
		}
	}

	return requests, manifest, nextManifest, nil
}

func inspectOwnedResourceAncestors(project, relative string) error {
	components := strings.Split(relative, "/")
	ancestor := project
	for _, component := range components[:len(components)-1] {
		ancestor = filepath.Join(ancestor, filepath.FromSlash(component))
		info, err := os.Lstat(ancestor)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("inspect ancestor of owned Resource path %q: %w", relative, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("owned Resource path %q has an unsafe ancestor", relative)
		}
	}

	return nil
}

func validateResourceJournal(journal addTransactionJournal) error {
	if len(journal.Installations) != 0 || len(journal.Removals) != 0 || journal.CreatedSkillsDirectory {
		return fmt.Errorf("project transaction journal is corrupt")
	}
	if journal.Operation == "resource-add" {
		if len(journal.ResourceInstallations) == 0 || len(journal.ResourceRemovals) != 0 || journal.NextManifest.Version != manifestVersion2 {
			return fmt.Errorf("project transaction journal is corrupt")
		}
	} else if len(journal.ResourceRemovals) == 0 || len(journal.ResourceInstallations) != 0 || len(journal.CreatedDirectories) != 0 || journal.CreatedAgentsDirectory || !journal.PreviousManifestExisted || journal.PreviousManifest.Version != manifestVersion2 || journal.NextManifest.Version != manifestVersion2 {
		return fmt.Errorf("project transaction journal is corrupt")
	}
	seenDestinations := make(map[string]bool)
	leaves := journal.ResourceInstallations
	if journal.Operation == "resource-remove" {
		leaves = journal.ResourceRemovals
	}
	for _, leaf := range leaves {
		if !validResourceName(leaf.Resource) || !safeProjectRelativePath(leaf.Destination) || !safeStageLeafPath(leaf.Source) || !leaf.Identity.recorded() && leaf.Present || journal.Operation == "resource-add" && !leaf.Present {
			return fmt.Errorf("project transaction journal is corrupt")
		}
		if seenDestinations[leaf.Destination] {
			return fmt.Errorf("project transaction journal is corrupt")
		}
		seenDestinations[leaf.Destination] = true
	}
	previous := ""
	for _, directory := range journal.CreatedDirectories {
		if !safeProjectRelativePath(directory.Destination) || previous != "" && strings.Count(directory.Destination, "/") < strings.Count(previous, "/") {
			return fmt.Errorf("project transaction journal is corrupt")
		}
		previous = directory.Destination
	}

	return nil
}

func safeStageLeafPath(path string) bool {
	return safeProjectRelativePath(path) && strings.HasPrefix(path, "leaves/")
}

func recoverInterruptedResourceTransaction(agentsDirectory string, journal addTransactionJournal, current projectManifest, currentExists bool) error {
	if currentExists && reflect.DeepEqual(current, journal.NextManifest) {
		return cleanupCommittedResourceTransaction(agentsDirectory, journal)
	}
	if currentExists != journal.PreviousManifestExisted || currentExists && !reflect.DeepEqual(current, journal.PreviousManifest) {
		return manualRecoveryError(agentsDirectory, "manifest no longer matches the transaction")
	}

	return rollbackResourceTransaction(agentsDirectory, journal, false)
}

func rollbackResourceTransaction(agentsDirectory string, journal addTransactionJournal, restoreManifest bool) error {
	project := filepath.Dir(agentsDirectory)
	if restoreManifest {
		if journal.PreviousManifestExisted {
			if err := writeProjectManifest(agentsDirectory, journal.PreviousManifest); err != nil {
				return err
			}
		} else if err := os.Remove(filepath.Join(agentsDirectory, "bond-manifest.json")); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	leaves := journal.ResourceInstallations
	isRemoval := journal.Operation == "resource-remove"
	if isRemoval {
		leaves = journal.ResourceRemovals
	} else if err := makeCreatedResourceDirectoriesAccessible(agentsDirectory, journal.CreatedDirectories, false); err != nil {
		return err
	}
	for _, leaf := range leaves {
		staged := filepath.Join(agentsDirectory, journal.StageDirectory, filepath.FromSlash(leaf.Source))
		destination := filepath.Join(project, filepath.FromSlash(leaf.Destination))
		stagedMatches := pathHasIdentity(staged, leaf.Identity)
		destinationMatches := pathHasIdentity(destination, leaf.Identity)
		if !leaf.Present {
			continue
		}
		if stagedMatches && !pathExists(destination) {
			if isRemoval {
				parentInfo, err := os.Lstat(filepath.Dir(destination))
				if err != nil || !parentInfo.IsDir() || parentInfo.Mode()&os.ModeSymlink != 0 {
					return manualRecoveryError(agentsDirectory, fmt.Sprintf("parent of Resource path %q changed during recovery", leaf.Destination))
				}
				if err := renameNoReplace(staged, destination); err != nil {
					return manualRecoveryError(agentsDirectory, fmt.Sprintf("restore Resource path %q: %v", leaf.Destination, err))
				}
				if err := syncDirectory(filepath.Dir(destination)); err != nil {
					return err
				}
				if err := syncDirectory(filepath.Dir(staged)); err != nil {
					return err
				}
			} else if err := os.Remove(staged); err != nil {
				return err
			} else if err := syncDirectory(filepath.Dir(staged)); err != nil {
				return err
			}
			continue
		}
		if destinationMatches && !pathExists(staged) {
			if isRemoval {
				continue
			}
			if err := os.Remove(destination); err != nil {
				return err
			}
			if err := syncDirectory(filepath.Dir(destination)); err != nil {
				return err
			}
			continue
		}
		return manualRecoveryError(agentsDirectory, fmt.Sprintf("Resource path %q conflicts with interrupted transaction", leaf.Destination))
	}
	if !isRemoval {
		for index := len(journal.CreatedDirectories) - 1; index >= 0; index-- {
			directory := journal.CreatedDirectories[index]
			path := filepath.Join(project, filepath.FromSlash(directory.Destination))
			if !directory.Identity.recorded() {
				if pathExists(path) {
					return manualRecoveryError(agentsDirectory, fmt.Sprintf("planned Resource directory %q appeared before Bond recorded its creation", directory.Destination))
				}
				continue
			}
			if !pathExists(path) {
				continue
			}
			if !pathHasIdentity(path, directory.Identity) {
				return manualRecoveryError(agentsDirectory, fmt.Sprintf("created Resource directory %q conflicts with interrupted transaction", directory.Destination))
			}
			if err := os.Remove(path); err != nil {
				return manualRecoveryError(agentsDirectory, fmt.Sprintf("created Resource directory %q is not empty", directory.Destination))
			}
			if err := syncDirectory(filepath.Dir(path)); err != nil {
				return err
			}
		}
	}
	if err := removeResourceStage(agentsDirectory, journal); err != nil {
		return err
	}
	if err := removeJournalDurably(agentsDirectory, journal); err != nil {
		return err
	}
	if journal.CreatedAgentsDirectory {
		if err := os.Remove(agentsDirectory); err != nil {
			return manualRecoveryError(agentsDirectory, "new Project infrastructure contains conflicting entries")
		}
		if err := syncDirectory(filepath.Dir(agentsDirectory)); err != nil {
			return err
		}
	}

	return nil
}

func cleanupCommittedResourceTransaction(agentsDirectory string, journal addTransactionJournal) error {
	project := filepath.Dir(agentsDirectory)
	if journal.Operation == "resource-add" {
		if err := makeCreatedResourceDirectoriesAccessible(agentsDirectory, journal.CreatedDirectories, true); err != nil {
			return err
		}
		for _, leaf := range journal.ResourceInstallations {
			destination := filepath.Join(project, filepath.FromSlash(leaf.Destination))
			if !pathHasIdentity(destination, leaf.Identity) {
				return manualRecoveryError(agentsDirectory, fmt.Sprintf("Resource path %q conflicts with committed transaction", leaf.Destination))
			}
			staged := filepath.Join(agentsDirectory, journal.StageDirectory, filepath.FromSlash(leaf.Source))
			if pathExists(staged) {
				return manualRecoveryError(agentsDirectory, fmt.Sprintf("Resource path %q was not fully published", leaf.Destination))
			}
		}
		if err := applyCreatedResourceDirectoryModes(agentsDirectory, journal.CreatedDirectories); err != nil {
			return err
		}
	} else {
		if err := removeCommittedResourceRemovalStage(agentsDirectory, journal); err != nil {
			return err
		}
	}
	if err := removeResourceStage(agentsDirectory, journal); err != nil {
		return err
	}

	return removeJournalDurably(agentsDirectory, journal)
}

func makeCreatedResourceDirectoriesAccessible(agentsDirectory string, directories []resourceJournalDirectory, requireAll bool) error {
	project := filepath.Dir(agentsDirectory)
	for _, directory := range directories {
		path := filepath.Join(project, filepath.FromSlash(directory.Destination))
		info, err := os.Lstat(path)
		if os.IsNotExist(err) && !requireAll {
			continue
		}
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return manualRecoveryError(agentsDirectory, fmt.Sprintf("created Resource directory %q is missing or unsafe", directory.Destination))
		}
		if !directory.Identity.recorded() {
			return manualRecoveryError(agentsDirectory, fmt.Sprintf("planned Resource directory %q appeared before Bond recorded its creation", directory.Destination))
		}
		if !pathHasIdentity(path, directory.Identity) {
			return manualRecoveryError(agentsDirectory, fmt.Sprintf("created Resource directory %q conflicts with the transaction", directory.Destination))
		}
		if err := os.Chmod(path, directory.Mode.Perm()|0o700); err != nil {
			return fmt.Errorf("make created Resource directory %q accessible during recovery: %w", directory.Destination, err)
		}
	}

	return nil
}

func applyCreatedResourceDirectoryModes(agentsDirectory string, directories []resourceJournalDirectory) error {
	project := filepath.Dir(agentsDirectory)
	for index := len(directories) - 1; index >= 0; index-- {
		directory := directories[index]
		path := filepath.Join(project, filepath.FromSlash(directory.Destination))
		if !pathHasIdentity(path, directory.Identity) {
			return manualRecoveryError(agentsDirectory, fmt.Sprintf("created Resource directory %q conflicts with the transaction", directory.Destination))
		}
		if err := syncDirectory(path); err != nil {
			return err
		}
		if err := os.Chmod(path, directory.Mode.Perm()); err != nil {
			return fmt.Errorf("set Resource directory permissions %q: %w", directory.Destination, err)
		}
		if err := syncDirectory(filepath.Dir(path)); err != nil {
			return err
		}
	}

	return nil
}

func removeCommittedResourceRemovalStage(agentsDirectory string, journal addTransactionJournal) error {
	project := filepath.Dir(agentsDirectory)
	stage := filepath.Join(agentsDirectory, journal.StageDirectory)
	_, stageErr := os.Lstat(stage)
	stageMissing := os.IsNotExist(stageErr)
	if stageErr != nil && !stageMissing {
		return stageErr
	}
	for _, leaf := range journal.ResourceRemovals {
		if pathExists(filepath.Join(project, filepath.FromSlash(leaf.Destination))) {
			return manualRecoveryError(agentsDirectory, fmt.Sprintf("Resource path %q conflicts with committed removal", leaf.Destination))
		}
		if !leaf.Present || stageMissing {
			continue
		}
		staged := filepath.Join(stage, filepath.FromSlash(leaf.Source))
		if _, err := os.Lstat(staged); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}
		if !pathHasIdentity(staged, leaf.Identity) {
			return manualRecoveryError(agentsDirectory, fmt.Sprintf("staged Resource path %q conflicts with committed removal", leaf.Destination))
		}
		if err := os.Remove(staged); err != nil {
			return err
		}
		if err := syncDirectory(filepath.Dir(staged)); err != nil {
			return err
		}
	}

	return removeResourceStage(agentsDirectory, journal)
}

func removeResourceStage(agentsDirectory string, journal addTransactionJournal) error {
	stage := filepath.Join(agentsDirectory, journal.StageDirectory)
	var paths []string
	err := filepath.WalkDir(stage, func(path string, entry os.DirEntry, walkErr error) error {
		if os.IsNotExist(walkErr) {
			return filepath.SkipDir
		}
		if walkErr != nil {
			return walkErr
		}
		if path != stage {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	sort.Slice(paths, func(i, j int) bool {
		return strings.Count(paths[i], string(os.PathSeparator)) > strings.Count(paths[j], string(os.PathSeparator))
	})
	for _, path := range paths {
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if info.IsDir() {
			if err := os.Remove(path); err != nil {
				return manualRecoveryError(agentsDirectory, "transaction staging contains conflicting entries")
			}
			continue
		}
		return manualRecoveryError(agentsDirectory, "transaction staging contains an unexpected leaf")
	}
	if err := os.Remove(stage); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := syncDirectory(agentsDirectory); err != nil {
		return err
	}

	return nil
}
