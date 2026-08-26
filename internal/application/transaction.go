package application

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"time"
)

const defaultProjectLockTimeout = 5 * time.Second

type projectLock struct {
	file *os.File
}

func acquireProjectLock(ctx context.Context, project string, timeout time.Duration) (*projectLock, error) {
	if timeout <= 0 || timeout > defaultProjectLockTimeout {
		timeout = defaultProjectLockTimeout
	}
	file, err := os.Open(project)
	if err != nil {
		return nil, fmt.Errorf("open Project %q for locking: %w", project, err)
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	retry := time.NewTicker(10 * time.Millisecond)
	defer retry.Stop()

	for {
		err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return &projectLock{file: file}, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			_ = file.Close()

			return nil, fmt.Errorf("lock Project %q: %w", project, err)
		}
		select {
		case <-ctx.Done():
			_ = file.Close()

			return nil, fmt.Errorf("lock Project %q: %w", project, ctx.Err())
		case <-deadline.C:
			_ = file.Close()

			return nil, fmt.Errorf("project %q is locked by another Bond process", project)
		case <-retry.C:
		}
	}
}

func (lock *projectLock) release() error {
	if err := syscall.Flock(int(lock.file.Fd()), syscall.LOCK_UN); err != nil {
		_ = lock.file.Close()

		return fmt.Errorf("unlock Project: %w", err)
	}
	if err := lock.file.Close(); err != nil {
		return fmt.Errorf("close Project lock: %w", err)
	}

	return nil
}

type addTransactionJournal struct {
	Version                 int                  `json:"version"`
	Operation               string               `json:"operation,omitempty"`
	StageDirectory          string               `json:"stageDirectory"`
	CreatedAgentsDirectory  bool                 `json:"createdAgentsDirectory"`
	CreatedSkillsDirectory  bool                 `json:"createdSkillsDirectory"`
	PreviousManifestExisted bool                 `json:"previousManifestExisted"`
	PreviousManifest        projectManifest      `json:"previousManifest"`
	NextManifest            projectManifest      `json:"nextManifest"`
	Installations           []stagedInstallation `json:"installations"`
	Removals                []stagedRemoval      `json:"removals,omitempty"`
}

type stagedRemoval struct {
	Name        string             `json:"name"`
	Mode        installationMode   `json:"mode"`
	Identity    filesystemIdentity `json:"identity,omitempty"`
	Destination string             `json:"destination"`
	Present     bool               `json:"present"`
}

type stagedInstallation struct {
	Name        string             `json:"name"`
	Source      string             `json:"source"`
	Mode        installationMode   `json:"mode,omitempty"`
	Identity    filesystemIdentity `json:"identity,omitempty"`
	Fingerprint skillFingerprint   `json:"fingerprint,omitempty"`
	Destination string             `json:"destination"`
}

func journalPath(agentsDirectory string) string {
	return filepath.Join(agentsDirectory, "bond-journal.json")
}

func writeAddJournal(agentsDirectory string, journal addTransactionJournal) error {
	contents, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		return fmt.Errorf("encode Project transaction journal: %w", err)
	}
	contents = append(contents, '\n')
	if err := writeAtomicFile(agentsDirectory, "bond-journal.json", ".bond-journal-*", contents); err != nil {
		return fmt.Errorf("write Project transaction journal: %w", err)
	}

	return nil
}

func readAddJournal(agentsDirectory string) (addTransactionJournal, bool, error) {
	path := journalPath(agentsDirectory)
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return addTransactionJournal{}, false, nil
	}
	if err != nil {
		return addTransactionJournal{}, false, fmt.Errorf("inspect Project transaction journal: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return addTransactionJournal{}, false, fmt.Errorf("project transaction journal must be a regular file, not a symlink")
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return addTransactionJournal{}, false, fmt.Errorf("read Project transaction journal: %w", err)
	}
	var journal addTransactionJournal
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&journal); err != nil {
		return addTransactionJournal{}, false, fmt.Errorf("project transaction journal is corrupt: %w", err)
	}
	if err := ensureJSONEnd(decoder); err != nil {
		return addTransactionJournal{}, false, fmt.Errorf("project transaction journal is corrupt: %w", err)
	}
	if journal.Version != 1 || filepath.Base(journal.StageDirectory) != journal.StageDirectory || !strings.HasPrefix(journal.StageDirectory, ".bond-stage-") {
		return addTransactionJournal{}, false, fmt.Errorf("project transaction journal is corrupt")
	}
	stageInfo, stageErr := os.Lstat(filepath.Join(agentsDirectory, journal.StageDirectory))
	if stageErr == nil {
		if stageInfo.Mode()&os.ModeSymlink != 0 {
			return addTransactionJournal{}, false, fmt.Errorf("project transaction staging must not be a symlink")
		}
		if !stageInfo.IsDir() {
			return addTransactionJournal{}, false, fmt.Errorf("project transaction staging must be a directory")
		}
	} else if !os.IsNotExist(stageErr) {
		return addTransactionJournal{}, false, fmt.Errorf("inspect Project transaction staging: %w", stageErr)
	}
	if journal.PreviousManifest.Version != manifestVersion || journal.PreviousManifest.Skills == nil || journal.NextManifest.Version != manifestVersion || journal.NextManifest.Skills == nil {
		return addTransactionJournal{}, false, fmt.Errorf("project transaction journal is corrupt")
	}
	if err := validateManifestRecords(journal.PreviousManifest.Skills); err != nil {
		return addTransactionJournal{}, false, fmt.Errorf("project transaction journal is corrupt: invalid previous manifest: %w", err)
	}
	if err := validateManifestRecords(journal.NextManifest.Skills); err != nil {
		return addTransactionJournal{}, false, fmt.Errorf("project transaction journal is corrupt: invalid next manifest: %w", err)
	}
	if journal.Operation != "" && journal.Operation != "remove" {
		return addTransactionJournal{}, false, fmt.Errorf("project transaction journal is corrupt")
	}
	if journal.Operation == "remove" {
		if len(journal.Installations) != 0 || len(journal.Removals) == 0 || journal.CreatedAgentsDirectory || journal.CreatedSkillsDirectory || !journal.PreviousManifestExisted {
			return addTransactionJournal{}, false, fmt.Errorf("project transaction journal is corrupt")
		}
		for _, removal := range journal.Removals {
			if !validProjectSkillBasename(removal.Name) || removal.Destination != filepath.ToSlash(filepath.Join("skills", removal.Name)) || (removal.Mode != linkMode && removal.Mode != copyMode) || (removal.Present != removal.Identity.recorded()) {
				return addTransactionJournal{}, false, fmt.Errorf("project transaction journal is corrupt")
			}
		}
	} else {
		if len(journal.Removals) != 0 {
			return addTransactionJournal{}, false, fmt.Errorf("project transaction journal is corrupt")
		}
		for index := range journal.Installations {
			installation := &journal.Installations[index]
			if installation.Mode == "" {
				installation.Mode = linkMode
			}
			if !validProjectSkillBasename(installation.Name) || !filepath.IsAbs(installation.Source) || installation.Destination != filepath.ToSlash(filepath.Join("skills", installation.Name)) {
				return addTransactionJournal{}, false, fmt.Errorf("project transaction journal is corrupt")
			}
			if installation.Mode != linkMode && installation.Mode != copyMode {
				return addTransactionJournal{}, false, fmt.Errorf("project transaction journal is corrupt")
			}
			if installation.Mode == linkMode && installation.Fingerprint != "" {
				return addTransactionJournal{}, false, fmt.Errorf("project transaction journal is corrupt")
			}
			if installation.Mode == copyMode && (!installation.Identity.recorded() || !installation.Fingerprint.valid()) {
				return addTransactionJournal{}, false, fmt.Errorf("project transaction journal is corrupt")
			}
		}
	}

	return journal, true, nil
}

func ensureNoInterruptedTransaction(project string) error {
	agentsDirectory := filepath.Join(project, ".agents")
	exists, err := realDirectoryIfPresent(agentsDirectory, ".agents")
	if err != nil || !exists {
		return err
	}
	_, interrupted, err := readAddJournal(agentsDirectory)
	if err != nil {
		return err
	}
	if interrupted {
		return fmt.Errorf("project transaction in %q requires recovery before this operation", project)
	}

	return nil
}

func recoverInterruptedAdd(agentsDirectory string) error {
	journal, exists, err := readAddJournal(agentsDirectory)
	if err != nil || !exists {
		return err
	}
	current, currentExists, err := readManifestState(agentsDirectory)
	if err != nil {
		return err
	}
	if journal.Operation == "remove" {
		return recoverInterruptedRemove(agentsDirectory, journal, current, currentExists)
	}
	if currentExists && reflect.DeepEqual(current, journal.NextManifest) {
		if err := verifyPublishedInstallations(agentsDirectory, journal); err != nil {
			return err
		}

		return cleanupCommittedAdd(agentsDirectory, journal)
	}
	if currentExists != journal.PreviousManifestExisted || (currentExists && !reflect.DeepEqual(current, journal.PreviousManifest)) {
		return manualRecoveryError(agentsDirectory, "manifest no longer matches the transaction")
	}

	return rollbackAddTransaction(agentsDirectory, journal, false)
}

func rollbackAddTransaction(agentsDirectory string, journal addTransactionJournal, restoreManifest bool) error {
	if err := validateRollbackPaths(agentsDirectory, journal); err != nil {
		return err
	}
	if restoreManifest {
		if err := restorePreviousManifest(agentsDirectory, journal); err != nil {
			return err
		}
	}
	stageDirectory := filepath.Join(agentsDirectory, journal.StageDirectory)
	for _, installation := range journal.Installations {
		staged := filepath.Join(stageDirectory, installation.Name)
		destination := filepath.Join(agentsDirectory, filepath.FromSlash(installation.Destination))
		_, stageErr := os.Lstat(staged)
		_, destinationErr := os.Lstat(destination)
		if stageErr == nil && destinationErr == nil {
			return manualRecoveryError(agentsDirectory, fmt.Sprintf("destination %q was created after interruption", installation.Destination))
		}
		if stageErr == nil {
			if err := removeInstalledPath(staged, installation.Mode); err != nil {
				return fmt.Errorf("remove staged Project Skill %q: %w", installation.Name, err)
			}
		} else if !os.IsNotExist(stageErr) {
			return manualRecoveryError(agentsDirectory, fmt.Sprintf("staged Project Skill %q cannot be inspected", installation.Name))
		}
		if !os.IsNotExist(destinationErr) {
			if destinationErr != nil {
				return manualRecoveryError(agentsDirectory, fmt.Sprintf("destination %q cannot be inspected", installation.Destination))
			}
			if !installationMatches(destination, installation) {
				return manualRecoveryError(agentsDirectory, fmt.Sprintf("destination %q conflicts with interrupted installation", installation.Destination))
			}
			if err := removeInstalledPath(destination, installation.Mode); err != nil {
				return fmt.Errorf("roll back Project Skill %q: %w", installation.Name, err)
			}
			if err := syncDirectory(filepath.Dir(destination)); err != nil {
				return err
			}
		}
	}
	if err := removeEmptyStageDirectory(agentsDirectory, journal); err != nil {
		return err
	}
	if journal.CreatedSkillsDirectory {
		if err := os.Remove(filepath.Join(agentsDirectory, "skills")); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove newly created Project infrastructure .agents/skills: %w", err)
		}
		if err := syncDirectory(agentsDirectory); err != nil {
			return err
		}
	}
	if err := removeJournalDurably(agentsDirectory, journal); err != nil {
		return err
	}
	if journal.CreatedAgentsDirectory {
		if err := os.Remove(agentsDirectory); err != nil {
			restoreErr := writeAddJournal(agentsDirectory, journal)

			return errors.Join(manualRecoveryError(agentsDirectory, "new Project infrastructure appeared after interruption"), restoreErr)
		}
		if err := syncDirectory(filepath.Dir(agentsDirectory)); err != nil {
			return err
		}

		return nil
	}
	return nil
}

func removeJournalDurably(agentsDirectory string, journal addTransactionJournal) error {
	if err := os.Remove(journalPath(agentsDirectory)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove Project transaction journal: %w", err)
	}
	if err := syncDirectory(agentsDirectory); err != nil {
		restoreErr := writeAddJournal(agentsDirectory, journal)

		return errors.Join(err, restoreErr)
	}

	return nil
}

func validateRollbackPaths(agentsDirectory string, journal addTransactionJournal) error {
	stageDirectory := filepath.Join(agentsDirectory, journal.StageDirectory)
	stageEntries, err := os.ReadDir(stageDirectory)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read Project transaction staging: %w", err)
	}
	installations := make(map[string]stagedInstallation, len(journal.Installations))
	for _, installation := range journal.Installations {
		installations[installation.Name] = installation
	}
	for _, entry := range stageEntries {
		installation, exists := installations[entry.Name()]
		if !exists {
			return manualRecoveryError(agentsDirectory, fmt.Sprintf("staging path %q was created after interruption", entry.Name()))
		}
		if !installationMatches(filepath.Join(stageDirectory, entry.Name()), installation) {
			return manualRecoveryError(agentsDirectory, fmt.Sprintf("staged Project Skill %q conflicts with the transaction", entry.Name()))
		}
	}
	skillsDirectory := filepath.Join(agentsDirectory, "skills")
	skillsExist, inspectErr := realDirectoryIfPresent(skillsDirectory, ".agents/skills")
	if inspectErr != nil {
		return inspectErr
	}
	if !journal.CreatedSkillsDirectory {
		if !skillsExist {
			return manualRecoveryError(agentsDirectory, "Project infrastructure .agents/skills disappeared after interruption")
		}

		return nil
	}
	var skillEntries []os.DirEntry
	if skillsExist {
		skillEntries, err = os.ReadDir(skillsDirectory)
		if err != nil {
			return fmt.Errorf("read newly created Project infrastructure .agents/skills: %w", err)
		}
	}
	for _, entry := range skillEntries {
		if _, exists := installations[entry.Name()]; !exists {
			return manualRecoveryError(agentsDirectory, fmt.Sprintf("Project Skill path %q was created after interruption", entry.Name()))
		}
	}
	if journal.CreatedAgentsDirectory {
		allowed := map[string]bool{
			"bond-journal.json":    true,
			"bond-manifest.json":   true,
			"skills":               true,
			journal.StageDirectory: true,
		}
		agentEntries, readErr := os.ReadDir(agentsDirectory)
		if readErr != nil {
			return fmt.Errorf("read newly created Project infrastructure .agents: %w", readErr)
		}
		for _, entry := range agentEntries {
			if !allowed[entry.Name()] {
				return manualRecoveryError(agentsDirectory, fmt.Sprintf("Project infrastructure path %q was created after interruption", entry.Name()))
			}
		}
	}

	return nil
}

func removeEmptyStageDirectory(agentsDirectory string, journal addTransactionJournal) error {
	stageDirectory := filepath.Join(agentsDirectory, journal.StageDirectory)
	entries, err := os.ReadDir(stageDirectory)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read Project transaction staging: %w", err)
	}
	if len(entries) != 0 {
		return manualRecoveryError(agentsDirectory, "transaction staging contains paths created after interruption")
	}
	if err := os.Remove(stageDirectory); err != nil {
		return fmt.Errorf("remove Project transaction staging: %w", err)
	}

	return syncDirectory(agentsDirectory)
}

func restorePreviousManifest(agentsDirectory string, journal addTransactionJournal) error {
	if !journal.PreviousManifestExisted {
		if err := os.Remove(filepath.Join(agentsDirectory, "bond-manifest.json")); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("restore absent Project manifest: %w", err)
		}

		return syncDirectory(agentsDirectory)
	}

	return writeProjectManifest(agentsDirectory, journal.PreviousManifest)
}

func verifyPublishedInstallations(agentsDirectory string, journal addTransactionJournal) error {
	for _, installation := range journal.Installations {
		destination := filepath.Join(agentsDirectory, filepath.FromSlash(installation.Destination))
		if !installationMatches(destination, installation) {
			return manualRecoveryError(agentsDirectory, fmt.Sprintf("committed destination %q conflicts with the transaction", installation.Destination))
		}
	}

	return nil
}

func installationMatches(path string, installation stagedInstallation) bool {
	if installation.Identity.recorded() {
		identity, err := identifyPath(path)
		if err != nil || identity != installation.Identity {
			return false
		}
	}
	if installation.Mode == copyMode {
		fingerprint, err := skillTreeFingerprint(path)

		return err == nil && fingerprint == installation.Fingerprint
	}
	target, err := os.Readlink(path)

	return err == nil && target == installation.Source
}

func cleanupCommittedAdd(agentsDirectory string, journal addTransactionJournal) error {
	if err := removeEmptyStageDirectory(agentsDirectory, journal); err != nil {
		return err
	}

	return removeJournalDurably(agentsDirectory, journal)
}

func readManifestState(agentsDirectory string) (projectManifest, bool, error) {
	_, err := os.Lstat(filepath.Join(agentsDirectory, "bond-manifest.json"))
	if os.IsNotExist(err) {
		return emptyProjectManifest(), false, nil
	}
	if err != nil {
		return projectManifest{}, false, fmt.Errorf("inspect Project manifest: %w", err)
	}
	manifest, err := readProjectManifest(agentsDirectory)

	return manifest, true, err
}

func manualRecoveryError(agentsDirectory, reason string) error {
	return fmt.Errorf("project transaction requires manual recovery in %q: %s; preserve bond-journal.json and staging paths, move the conflicting path aside, then rerun Bond", agentsDirectory, reason)
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open directory %q for synchronization: %w", path, err)
	}
	if err := directory.Sync(); err != nil {
		_ = directory.Close()

		return fmt.Errorf("synchronize directory %q: %w", path, err)
	}
	if err := directory.Close(); err != nil {
		return fmt.Errorf("close synchronized directory %q: %w", path, err)
	}

	return nil
}

func writeAtomicFile(directory, name, pattern string, contents []byte) error {
	temporary, err := os.CreateTemp(directory, pattern)
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	if err := temporary.Chmod(0o644); err != nil {
		_ = temporary.Close()

		return removeTemporaryManifest(temporaryPath, err)
	}
	if _, err := temporary.Write(contents); err != nil {
		_ = temporary.Close()

		return removeTemporaryManifest(temporaryPath, err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()

		return removeTemporaryManifest(temporaryPath, err)
	}
	if err := temporary.Close(); err != nil {
		return removeTemporaryManifest(temporaryPath, err)
	}
	if err := os.Rename(temporaryPath, filepath.Join(directory, name)); err != nil {
		return removeTemporaryManifest(temporaryPath, err)
	}

	return syncDirectory(directory)
}
