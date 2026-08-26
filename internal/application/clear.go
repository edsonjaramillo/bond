package application

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func clearSkills(command *cobra.Command, invocation Invocation) (resultError error) {
	lock, err := acquireProjectLock(command.Context(), invocation.WorkingDirectory, invocation.projectLockTimeout)
	if err != nil {
		return err
	}
	defer func() {
		resultError = errors.Join(resultError, lock.release())
	}()

	agentsDirectory := filepath.Join(invocation.WorkingDirectory, ".agents")
	agentsExists, err := realDirectoryIfPresent(agentsDirectory, ".agents")
	if err != nil || !agentsExists {
		return err
	}
	manifest, manifestExists, err := readManifestState(agentsDirectory)
	if err != nil {
		return err
	}
	journal, journalExists, err := readAddJournal(agentsDirectory)
	if err != nil {
		return err
	}
	if err := ensureNoOrphanedProjectStages(agentsDirectory, journal, journalExists); err != nil {
		return err
	}
	if !manifestExists && !journalExists {
		return nil
	}
	if manifestExists {
		names := managedSkillNames(manifest)
		if _, _, _, err := preflightRemovals(invocation.WorkingDirectory, names); err != nil {
			return err
		}
	}
	if err := recoverInterruptedAdd(agentsDirectory); err != nil {
		return err
	}
	manifest, manifestExists, err = readManifestState(agentsDirectory)
	if err != nil || !manifestExists || len(manifest.Skills) == 0 {
		return err
	}
	names := managedSkillNames(manifest)
	requests, previousManifest, nextManifest, err := preflightRemovals(invocation.WorkingDirectory, names)
	if err != nil {
		return err
	}

	return removePreflightedSkills(invocation, requests, previousManifest, nextManifest)
}

func ensureNoOrphanedProjectStages(agentsDirectory string, journal addTransactionJournal, journalExists bool) error {
	entries, err := os.ReadDir(agentsDirectory)
	if err != nil {
		return fmt.Errorf("read Project infrastructure .agents: %w", err)
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), ".bond-stage-") {
			continue
		}
		if journalExists && entry.Name() == journal.StageDirectory {
			continue
		}

		return fmt.Errorf("project transaction has unresolved staging %q", entry.Name())
	}

	return nil
}

func managedSkillNames(manifest projectManifest) []string {
	names := make([]string, 0, len(manifest.Skills))
	for _, record := range manifest.Skills {
		names = append(names, record.Name)
	}

	return names
}
