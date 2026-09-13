package application

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type resourceLeaf struct {
	relative string
	source   string
	mode     os.FileMode
	link     string
}

type storedResource struct {
	name        string
	root        string
	leaves      []resourceLeaf
	directories map[string]os.FileMode
	errors      []string
}

func resourceStorePath(environment []string) (string, error) {
	return centralCollectionPath(environment, resourceCollection)
}

func validResourceName(name string) bool {
	if name == "" || strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") || strings.Contains(name, "--") {
		return false
	}
	for _, character := range name {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' {
			return false
		}
	}

	return true
}

func inspectStoredResource(environment []string, name string) (storedResource, error) {
	resource := storedResource{name: name, directories: make(map[string]os.FileMode)}
	if !validResourceName(name) {
		return resource, fmt.Errorf("resource name must use lowercase kebab-case")
	}
	store, err := resourceStorePath(environment)
	if err != nil {
		return resource, err
	}
	root := filepath.Join(store, name)
	if err := requireRealDirectory(root, fmt.Sprintf("selected Stored Resource %q", name)); err != nil {
		return resource, err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return resource, fmt.Errorf("resolve Stored Resource %q: %w", name, err)
	}
	resource.root = root
	leafSet := make(map[string]bool)
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("inspect Resource entry %q: %w", relativeTreePath(root, path), walkErr)
		}
		if path == root {
			return nil
		}
		relative := relativeTreePath(root, path)
		info, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("inspect Resource entry %q: %w", relative, err)
		}
		switch {
		case info.IsDir():
			resource.directories[relative] = info.Mode().Perm()
		case info.Mode().IsRegular():
			resource.leaves = append(resource.leaves, resourceLeaf{relative: relative, source: path, mode: info.Mode().Perm()})
			leafSet[relative] = true
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("read Resource symlink %q: %w", relative, err)
			}
			if filepath.IsAbs(target) {
				return fmt.Errorf("resource symlink %q must have a safe relative target", relative)
			}
			resolved := filepath.Clean(filepath.Join(filepath.Dir(relative), target))
			if resolved == "." || resolved == ".." || strings.HasPrefix(filepath.ToSlash(resolved), "../") {
				return fmt.Errorf("resource symlink %q must not escape the Resource", relative)
			}
			resource.leaves = append(resource.leaves, resourceLeaf{relative: relative, source: path, mode: info.Mode(), link: target})
			leafSet[relative] = true
		default:
			return fmt.Errorf("resource entry %q must be a directory, regular file, or safe relative symlink", relative)
		}

		return nil
	})
	if err != nil {
		return resource, err
	}
	if len(resource.leaves) == 0 {
		return resource, fmt.Errorf("stored Resource %q has no installable leaf entries", name)
	}
	symlinkTargets := make(map[string]string)
	for _, leaf := range resource.leaves {
		if leaf.link == "" {
			continue
		}
		resolved := filepath.ToSlash(filepath.Clean(filepath.Join(filepath.Dir(leaf.relative), leaf.link)))
		if !leafSet[resolved] {
			return resource, fmt.Errorf("resource symlink %q targets %q, which is not an installed leaf", leaf.relative, leaf.link)
		}
		symlinkTargets[filepath.ToSlash(leaf.relative)] = resolved
	}
	if cyclePath := resourceSymlinkCycle(symlinkTargets); cyclePath != "" {
		return resource, fmt.Errorf("resource symlink graph contains a cycle at %q", cyclePath)
	}

	return resource, nil
}

func resourceSymlinkCycle(targets map[string]string) string {
	const (
		visiting = 1
		visited  = 2
	)
	states := make(map[string]int, len(targets))
	var visit func(string) string
	visit = func(path string) string {
		switch states[path] {
		case visiting:
			return path
		case visited:
			return ""
		}
		states[path] = visiting
		if target, exists := targets[path]; exists {
			if cycle := visit(target); cycle != "" {
				return cycle
			}
		}
		states[path] = visited

		return ""
	}
	paths := make([]string, 0, len(targets))
	for path := range targets {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		if cycle := visit(path); cycle != "" {
			return cycle
		}
	}

	return ""
}

func validateResourceManifestRecords(records []managedResourceRecord) error {
	names := make(map[string]bool)
	var paths []string
	for index, record := range records {
		if !validResourceName(record.Name) {
			return fmt.Errorf("resource record %d has invalid Resource Name %q", index, record.Name)
		}
		if names[record.Name] {
			return fmt.Errorf("recorded Resource Name %q is repeated", record.Name)
		}
		if record.Mode != linkMode && record.Mode != copyMode {
			return fmt.Errorf("resource record %d has unsupported mode %q", index, record.Mode)
		}
		if len(record.Paths) == 0 {
			return fmt.Errorf("resource record %d paths must be a non-empty array", index)
		}
		previous := ""
		for _, path := range record.Paths {
			if !safeProjectRelativePath(path) {
				return fmt.Errorf("resource record %d has invalid path %q", index, path)
			}
			if path <= previous {
				return fmt.Errorf("resource record %d paths must be unique and sorted", index)
			}
			for _, existingPath := range paths {
				if pathsOverlap(path, existingPath) {
					return fmt.Errorf("managed Resource paths %q and %q overlap", existingPath, path)
				}
			}
			paths = append(paths, path)
			previous = path
		}
		names[record.Name] = true
	}

	return nil
}

func validateCrossKindManifestRecords(skills []managedSkillRecord, resources []managedResourceRecord) error {
	for _, skillRecord := range skills {
		for _, resourceRecord := range resources {
			for _, resourcePath := range resourceRecord.Paths {
				if pathsOverlap(skillRecord.Destination, resourcePath) {
					return fmt.Errorf("managed Resource %q path %q overlaps Managed Skill %q destination %q", resourceRecord.Name, resourcePath, skillRecord.Name, skillRecord.Destination)
				}
			}
		}
	}

	return nil
}

func safeProjectRelativePath(path string) bool {
	return path != "" && path != "." && !filepath.IsAbs(path) && filepath.ToSlash(filepath.Clean(filepath.FromSlash(path))) == path && path != ".." && !strings.HasPrefix(path, "../")
}

func validateJournalManifest(manifest projectManifest) error {
	if manifest.Version != manifestVersion1 && manifest.Version != manifestVersion2 || manifest.Skills == nil {
		return fmt.Errorf("unsupported manifest shape")
	}
	if manifest.Version == manifestVersion1 && manifest.Resources != nil || manifest.Version == manifestVersion2 && manifest.Resources == nil {
		return fmt.Errorf("unsupported manifest shape")
	}
	if err := validateManifestRecords(manifest.Skills); err != nil {
		return err
	}
	if err := validateResourceManifestRecords(manifest.Resources); err != nil {
		return err
	}

	return validateCrossKindManifestRecords(manifest.Skills, manifest.Resources)
}

func resourcePathReserved(path string) bool {
	for _, metadataPath := range []string{".agents/bond-manifest.json", ".agents/bond-journal.json"} {
		if pathsOverlap(path, metadataPath) {
			return true
		}
	}
	return strings.HasPrefix(path, ".agents/.bond-")
}

func pathsOverlap(first, second string) bool {
	return first == second || strings.HasPrefix(first, second+"/") || strings.HasPrefix(second, first+"/")
}

func preflightResources(invocation Invocation, arguments []string) ([]storedResource, projectManifest, bool, bool, error) {
	requests := make([]storedResource, len(arguments))
	seenNames := make(map[string]bool)
	for index, argument := range arguments {
		request, err := inspectStoredResource(invocation.Environment, argument)
		if err != nil {
			request.name = argument
			request.errors = append(request.errors, err.Error())
		}
		if seenNames[argument] {
			request.errors = append(request.errors, "Resource Name is repeated")
		}
		seenNames[argument] = true
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
	managedNames := make(map[string]bool)
	type ownedResourcePath struct {
		path  string
		owner string
	}
	var managedPaths []ownedResourcePath
	for _, record := range manifest.Resources {
		managedNames[record.Name] = true
		for _, path := range record.Paths {
			managedPaths = append(managedPaths, ownedResourcePath{path: path, owner: record.Name})
		}
	}
	var requestedPaths []ownedResourcePath
	transactionPath := invocation.WorkingDirectory
	if agentsExists {
		transactionPath = agentsDirectory
	}
	transactionIdentity, transactionIdentityError := identifyPath(transactionPath)
	for index := range requests {
		request := &requests[index]
		if managedNames[request.name] {
			request.errors = append(request.errors, fmt.Sprintf("ownership for Managed Resource %q already exists", request.name))
		}
		for _, leaf := range request.leaves {
			path := filepath.ToSlash(leaf.relative)
			for _, requestedPath := range requestedPaths {
				if pathsOverlap(path, requestedPath.path) {
					request.errors = append(request.errors, fmt.Sprintf("path %q collides with Resource %q path %q", path, requestedPath.owner, requestedPath.path))
				}
			}
			requestedPaths = append(requestedPaths, ownedResourcePath{path: path, owner: request.name})
			for _, managedPath := range managedPaths {
				if pathsOverlap(path, managedPath.path) {
					request.errors = append(request.errors, fmt.Sprintf("path %q overlaps path %q owned by Managed Resource %q", path, managedPath.path, managedPath.owner))
				}
			}
			if resourcePathReserved(path) {
				request.errors = append(request.errors, fmt.Sprintf("path %q is reserved for Bond metadata", path))
			}
			for _, skillRecord := range manifest.Skills {
				if pathsOverlap(path, skillRecord.Destination) {
					request.errors = append(request.errors, fmt.Sprintf("path %q overlaps Managed Skill %q", path, skillRecord.Name))
				}
			}
			destinationError := inspectResourceDestination(invocation.WorkingDirectory, path)
			if destinationError != nil {
				request.errors = append(request.errors, destinationError.Error())
			}
			if transactionIdentityError == nil && destinationError == nil {
				device, err := destinationFilesystemDevice(invocation.WorkingDirectory, path)
				if err != nil {
					request.errors = append(request.errors, err.Error())
				} else if device != transactionIdentity.Device {
					request.errors = append(request.errors, fmt.Sprintf("destination %q is on a different filesystem from the Project transaction area", path))
				}
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
	if len(diagnostics) > 0 {
		return nil, projectManifest{}, false, false, fmt.Errorf("%s", strings.Join(diagnostics, "\n"))
	}

	return requests, manifest, manifestExists, agentsExists, nil
}

func inspectResourceDestination(project, relative string) error {
	components := strings.Split(relative, "/")
	ancestor := project
	for _, component := range components[:len(components)-1] {
		ancestor = filepath.Join(ancestor, filepath.FromSlash(component))
		info, err := os.Lstat(ancestor)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("inspect ancestor of destination %q: %w", relative, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("destination %q has a symlinked ancestor", relative)
		}
		if !info.IsDir() {
			return fmt.Errorf("destination %q has a non-directory ancestor", relative)
		}
	}
	destination := filepath.Join(ancestor, filepath.FromSlash(components[len(components)-1]))
	if _, err := os.Lstat(destination); err == nil {
		return fmt.Errorf("destination %q already exists", relative)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect destination %q: %w", relative, err)
	}

	return nil
}

func destinationFilesystemDevice(project, relative string) (uint64, error) {
	ancestor := filepath.Dir(filepath.Join(project, filepath.FromSlash(relative)))
	for {
		identity, err := identifyPath(ancestor)
		if err == nil {
			return identity.Device, nil
		}
		if !os.IsNotExist(err) {
			return 0, fmt.Errorf("inspect destination filesystem for %q: %w", relative, err)
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return 0, fmt.Errorf("resolve destination filesystem for %q", relative)
		}
		ancestor = parent
	}
}

func addResources(command *cobra.Command, invocation Invocation, arguments []string, mode installationMode) (resultError error) {
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
	requests, manifest, manifestExists, agentsExists, err := preflightResources(invocation, arguments)
	if err != nil {
		return err
	}
	createdAgents := false
	if !agentsExists {
		if err := os.Mkdir(agentsDirectory, 0o755); err != nil {
			return fmt.Errorf("create Project infrastructure .agents: %w", err)
		}
		createdAgents = true
		if err := syncDirectory(invocation.WorkingDirectory); err != nil {
			return errors.Join(err, os.Remove(agentsDirectory))
		}
	}
	stageDirectory, err := os.MkdirTemp(agentsDirectory, ".bond-stage-")
	if err != nil {
		if createdAgents {
			_ = os.Remove(agentsDirectory)
		}
		return fmt.Errorf("create Project transaction staging: %w", err)
	}
	if err := syncDirectory(agentsDirectory); err != nil {
		return errors.Join(err, cleanupUnjournaledResourceAdd(agentsDirectory, stageDirectory, createdAgents))
	}

	nextManifest := manifest
	if nextManifest.Version == manifestVersion1 {
		nextManifest.Version = manifestVersion2
		nextManifest.Resources = []managedResourceRecord{}
	}
	journal := addTransactionJournal{
		Version: 1, Operation: "resource-add", StageDirectory: filepath.Base(stageDirectory),
		CreatedAgentsDirectory: createdAgents, PreviousManifestExisted: manifestExists,
		PreviousManifest: manifest, NextManifest: nextManifest,
	}
	createdDirectoryModes := make(map[string]os.FileMode)
	leafIndex := 0
	for _, request := range requests {
		paths := make([]string, 0, len(request.leaves))
		for _, leaf := range request.leaves {
			destination := filepath.ToSlash(leaf.relative)
			paths = append(paths, destination)
			for directory := filepath.Dir(leaf.relative); directory != "."; directory = filepath.Dir(directory) {
				relativeDirectory := filepath.ToSlash(directory)
				if _, exists := createdDirectoryModes[relativeDirectory]; !exists {
					createdDirectoryModes[relativeDirectory] = request.directories[relativeDirectory]
				}
			}
			stagedRelative := filepath.ToSlash(filepath.Join("leaves", fmt.Sprintf("%06d", leafIndex)))
			staged := filepath.Join(stageDirectory, filepath.FromSlash(stagedRelative))
			if err := os.MkdirAll(filepath.Dir(staged), 0o755); err != nil {
				return errors.Join(err, cleanupUnjournaledResourceAdd(agentsDirectory, stageDirectory, createdAgents))
			}
			if mode == copyMode && leaf.link == "" {
				err = copyRegularFile(leaf.source, staged, leaf.mode.Perm())
			} else if leaf.link != "" {
				err = os.Symlink(leaf.link, staged)
			} else {
				err = os.Symlink(leaf.source, staged)
			}
			if err != nil {
				return errors.Join(fmt.Errorf("stage Resource path %q: %w", destination, err), cleanupUnjournaledResourceAdd(agentsDirectory, stageDirectory, createdAgents))
			}
			identity, identifyErr := identifyPath(staged)
			if identifyErr != nil {
				return errors.Join(identifyErr, cleanupUnjournaledResourceAdd(agentsDirectory, stageDirectory, createdAgents))
			}
			journal.ResourceInstallations = append(journal.ResourceInstallations, resourceJournalLeaf{
				Resource: request.name, Source: stagedRelative, Destination: destination, Identity: identity, Present: true,
			})
			leafIndex++
		}
		sort.Strings(paths)
		nextManifest.Resources = append(nextManifest.Resources, managedResourceRecord{Name: request.name, Mode: mode, Paths: paths})
	}
	journal.NextManifest = nextManifest
	journal.CreatedDirectories = resourceDirectoriesToCreate(invocation.WorkingDirectory, createdDirectoryModes, mode)
	if err := syncDirectory(filepath.Join(stageDirectory, "leaves")); err != nil {
		return errors.Join(err, cleanupUnjournaledResourceAdd(agentsDirectory, stageDirectory, createdAgents))
	}
	if err := syncDirectory(stageDirectory); err != nil {
		return errors.Join(err, cleanupUnjournaledResourceAdd(agentsDirectory, stageDirectory, createdAgents))
	}
	if err := writeAddJournal(agentsDirectory, journal); err != nil {
		if _, journalError := os.Lstat(journalPath(agentsDirectory)); journalError == nil {
			return errors.Join(err, rollbackResourceTransaction(agentsDirectory, journal, false))
		}
		return errors.Join(err, cleanupUnjournaledResourceAdd(agentsDirectory, stageDirectory, createdAgents))
	}
	if invocation.transactionHook != nil {
		if err := invocation.transactionHook(afterJournalWrite); err != nil {
			return errors.Join(fmt.Errorf("run Project transaction hook: %w", err), rollbackResourceTransaction(agentsDirectory, journal, false))
		}
	}
	if invocation.transactionInterruptionPoint == afterJournalWrite {
		return fmt.Errorf("interrupted Project transaction after journal write")
	}
	if err := publishResourceAdd(invocation, agentsDirectory, &journal); err != nil {
		if invocation.transactionInterruptionPoint == afterFirstPublish || invocation.transactionInterruptionPoint == afterAllPublishes {
			return err
		}
		return errors.Join(err, rollbackResourceTransaction(agentsDirectory, journal, false))
	}
	if err := writeProjectManifest(agentsDirectory, nextManifest); err != nil {
		return errors.Join(err, rollbackResourceTransaction(agentsDirectory, journal, true))
	}
	if invocation.transactionInterruptionPoint == afterManifestWrite {
		return fmt.Errorf("interrupted Project transaction after manifest write")
	}
	if invocation.transactionInterruptionPoint == afterStageRemoval {
		if err := removeResourceStage(agentsDirectory, journal); err != nil {
			return err
		}
		return fmt.Errorf("interrupted Project transaction after staging removal")
	}

	if err := cleanupCommittedResourceTransaction(agentsDirectory, journal); err != nil {
		return errors.Join(err, rollbackResourceTransaction(agentsDirectory, journal, true))
	}

	return nil
}

func publishResourceAdd(invocation Invocation, agentsDirectory string, journal *addTransactionJournal) error {
	project := invocation.WorkingDirectory
	transactionIdentity, err := identifyPath(agentsDirectory)
	if err != nil {
		return fmt.Errorf("identify Project transaction area: %w", err)
	}
	for index := range journal.CreatedDirectories {
		directory := &journal.CreatedDirectories[index]
		if err := inspectResourceDestination(project, directory.Destination); err != nil {
			return fmt.Errorf("revalidate Resource directory: %w", err)
		}
		path := filepath.Join(project, filepath.FromSlash(directory.Destination))
		parentIdentity, err := identifyPath(filepath.Dir(path))
		if err != nil {
			return fmt.Errorf("identify destination filesystem for %q: %w", directory.Destination, err)
		}
		if parentIdentity.Device != transactionIdentity.Device {
			return fmt.Errorf("destination %q is on a different filesystem from the Project transaction area", directory.Destination)
		}
		if err := os.Mkdir(path, directory.Mode.Perm()|0o700); err != nil {
			return fmt.Errorf("create Resource directory %q: %w", directory.Destination, err)
		}
		directory.Identity, err = identifyPath(path)
		if err != nil {
			return fmt.Errorf("identify created Resource directory %q: %w", directory.Destination, err)
		}
		if err := syncDirectory(filepath.Dir(path)); err != nil {
			return err
		}
		if err := writeAddJournal(agentsDirectory, *journal); err != nil {
			return err
		}
	}
	for index, leaf := range journal.ResourceInstallations {
		if err := inspectResourceDestination(project, leaf.Destination); err != nil {
			return fmt.Errorf("revalidate Resource destination: %w", err)
		}
		destination := filepath.Join(project, filepath.FromSlash(leaf.Destination))
		parentIdentity, err := identifyPath(filepath.Dir(destination))
		if err != nil {
			return fmt.Errorf("identify destination filesystem for %q: %w", leaf.Destination, err)
		}
		if parentIdentity.Device != transactionIdentity.Device {
			return fmt.Errorf("destination %q is on a different filesystem from the Project transaction area", leaf.Destination)
		}
		staged := filepath.Join(agentsDirectory, journal.StageDirectory, filepath.FromSlash(leaf.Source))
		if err := renameNoReplace(staged, destination); err != nil {
			return fmt.Errorf("publish Resource path %q without replacing an existing destination: %w", leaf.Destination, err)
		}
		if err := syncDirectory(filepath.Dir(destination)); err != nil {
			return err
		}
		if err := syncDirectory(filepath.Dir(staged)); err != nil {
			return err
		}
		if index == 0 && invocation.transactionInterruptionPoint == afterFirstPublish {
			return fmt.Errorf("interrupted Project transaction after first publication")
		}
		if index == 0 && invocation.transactionFailurePoint == afterFirstPublish {
			return fmt.Errorf("publish Resource batch: injected failure")
		}
	}
	if invocation.transactionInterruptionPoint == afterAllPublishes {
		return fmt.Errorf("interrupted Project transaction after all publications")
	}

	return nil
}

func resourceDirectoriesToCreate(project string, modes map[string]os.FileMode, mode installationMode) []resourceJournalDirectory {
	paths := make([]string, 0, len(modes))
	for path := range modes {
		paths = append(paths, path)
	}
	sort.Slice(paths, func(i, j int) bool {
		firstDepth := strings.Count(paths[i], "/")
		secondDepth := strings.Count(paths[j], "/")
		if firstDepth == secondDepth {
			return paths[i] < paths[j]
		}
		return firstDepth < secondDepth
	})
	var directories []resourceJournalDirectory
	for _, path := range paths {
		if _, err := os.Lstat(filepath.Join(project, filepath.FromSlash(path))); !os.IsNotExist(err) {
			continue
		}
		permissions := os.FileMode(0o755)
		if mode == copyMode {
			permissions = modes[path]
		}
		directories = append(directories, resourceJournalDirectory{Destination: path, Mode: permissions})
	}

	return directories
}

func cleanupUnjournaledResourceAdd(agentsDirectory, stageDirectory string, createdAgents bool) error {
	var result error
	if err := os.RemoveAll(stageDirectory); err != nil {
		result = err
	} else if err := syncDirectory(agentsDirectory); err != nil {
		result = err
	}
	if createdAgents {
		if err := os.Remove(agentsDirectory); err != nil {
			result = errors.Join(result, err)
		} else if err := syncDirectory(filepath.Dir(agentsDirectory)); err != nil {
			result = errors.Join(result, err)
		}
	}

	return result
}
