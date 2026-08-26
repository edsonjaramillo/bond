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
	StageDirectory          string               `json:"stageDirectory"`
	CreatedAgentsDirectory  bool                 `json:"createdAgentsDirectory"`
	CreatedSkillsDirectory  bool                 `json:"createdSkillsDirectory"`
	PreviousManifestExisted bool                 `json:"previousManifestExisted"`
	PreviousManifest        projectManifest      `json:"previousManifest"`
	NextManifest            projectManifest      `json:"nextManifest"`
	Installations           []stagedInstallation `json:"installations"`
}

type stagedInstallation struct {
	Name        string `json:"name"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
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
	if journal.PreviousManifest.Version != manifestVersion || journal.PreviousManifest.Skills == nil || journal.NextManifest.Version != manifestVersion || journal.NextManifest.Skills == nil {
		return addTransactionJournal{}, false, fmt.Errorf("project transaction journal is corrupt")
	}
	if err := validateManifestRecords(journal.PreviousManifest.Skills); err != nil {
		return addTransactionJournal{}, false, fmt.Errorf("project transaction journal is corrupt: invalid previous manifest: %w", err)
	}
	if err := validateManifestRecords(journal.NextManifest.Skills); err != nil {
		return addTransactionJournal{}, false, fmt.Errorf("project transaction journal is corrupt: invalid next manifest: %w", err)
	}
	for _, installation := range journal.Installations {
		if !validProjectSkillBasename(installation.Name) || !filepath.IsAbs(installation.Source) || installation.Destination != filepath.ToSlash(filepath.Join("skills", installation.Name)) {
			return addTransactionJournal{}, false, fmt.Errorf("project transaction journal is corrupt")
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
		if !os.IsNotExist(destinationErr) {
			if destinationErr != nil {
				return manualRecoveryError(agentsDirectory, fmt.Sprintf("destination %q cannot be inspected", installation.Destination))
			}
			target, err := os.Readlink(destination)
			if err != nil || target != installation.Source {
				return manualRecoveryError(agentsDirectory, fmt.Sprintf("destination %q conflicts with interrupted installation", installation.Destination))
			}
			if err := os.Remove(destination); err != nil {
				return fmt.Errorf("roll back Project Skill %q: %w", installation.Name, err)
			}
			if err := syncDirectory(filepath.Dir(destination)); err != nil {
				return err
			}
		}
	}
	if err := os.RemoveAll(stageDirectory); err != nil {
		return fmt.Errorf("remove Project transaction staging: %w", err)
	}
	if err := syncDirectory(agentsDirectory); err != nil {
		return err
	}
	if err := os.Remove(journalPath(agentsDirectory)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove Project transaction journal: %w", err)
	}
	if err := syncDirectory(agentsDirectory); err != nil {
		return err
	}
	if journal.CreatedSkillsDirectory {
		if err := os.Remove(filepath.Join(agentsDirectory, "skills")); err != nil {
			return fmt.Errorf("remove newly created Project infrastructure .agents/skills: %w", err)
		}
		if err := syncDirectory(agentsDirectory); err != nil {
			return err
		}
	}
	if journal.CreatedAgentsDirectory {
		if err := os.Remove(agentsDirectory); err != nil {
			return fmt.Errorf("remove newly created Project infrastructure .agents: %w", err)
		}
		if err := syncDirectory(filepath.Dir(agentsDirectory)); err != nil {
			return err
		}
	}

	return nil
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
		target, err := os.Readlink(destination)
		if err != nil || target != installation.Source {
			return manualRecoveryError(agentsDirectory, fmt.Sprintf("committed destination %q conflicts with the transaction", installation.Destination))
		}
	}

	return nil
}

func cleanupCommittedAdd(agentsDirectory string, journal addTransactionJournal) error {
	if err := os.RemoveAll(filepath.Join(agentsDirectory, journal.StageDirectory)); err != nil {
		return fmt.Errorf("remove Project transaction staging: %w", err)
	}
	if err := syncDirectory(agentsDirectory); err != nil {
		return err
	}
	if err := os.Remove(journalPath(agentsDirectory)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove Project transaction journal: %w", err)
	}

	return syncDirectory(agentsDirectory)
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
	return fmt.Errorf("project transaction requires manual recovery in %q: %s; preserve bond-journal.json and staging paths", agentsDirectory, reason)
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
