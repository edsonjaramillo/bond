package application

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/spf13/cobra"
)

const (
	instructionAfterWrite = "instruction-after-write"
	instructionBeforeSync = "instruction-before-sync"
)

func appendInstruction(command *cobra.Command, invocation Invocation, instructionPath string) error {
	name, err := parseTopLevelInstructionPath(instructionPath)
	if err != nil {
		return err
	}
	store, err := centralCollectionPath(invocation.Environment, instructionCollection)
	if err != nil {
		return err
	}
	contents, err := readInstruction(filepath.Join(store, name), instructionPath)
	if err != nil {
		return err
	}

	targetPath := filepath.Join(invocation.WorkingDirectory, "AGENTS.md")
	target, created, err := openInstructionTarget(targetPath)
	if err != nil {
		return err
	}
	locked := false
	rollbackInstructionAppend := func(operationErr error, originalLength int64, mutated bool) error {
		var rollbackErr error
		if created {
			// Unlink while the lock is still held. A cooperative waiter will then
			// reject the stale descriptor after it acquires the lock.
			if removeErr := os.Remove(targetPath); removeErr != nil && !os.IsNotExist(removeErr) {
				rollbackErr = errors.Join(rollbackErr, fmt.Errorf("remove newly created target %q: %w", targetPath, removeErr))
			} else if removeErr == nil {
				rollbackErr = errors.Join(rollbackErr, syncDirectory(filepath.Dir(targetPath)))
			}
			if locked {
				rollbackErr = errors.Join(rollbackErr, releaseInstructionTargetLock(target))
				locked = false
			} else {
				rollbackErr = errors.Join(rollbackErr, target.Close())
			}
		} else {
			if mutated {
				if truncateErr := target.Truncate(originalLength); truncateErr != nil {
					rollbackErr = errors.Join(rollbackErr, fmt.Errorf("restore target %q length: %w", targetPath, truncateErr))
				} else if syncErr := target.Sync(); syncErr != nil {
					rollbackErr = errors.Join(rollbackErr, fmt.Errorf("synchronize restored target %q: %w", targetPath, syncErr))
				}
			}
			if locked {
				rollbackErr = errors.Join(rollbackErr, releaseInstructionTargetLock(target))
				locked = false
			} else {
				rollbackErr = errors.Join(rollbackErr, target.Close())
			}
		}

		return errors.Join(operationErr, rollbackErr)
	}

	if err := acquireInstructionTargetLock(command.Context(), target, targetPath, invocation.projectLockTimeout); err != nil {
		return rollbackInstructionAppend(err, 0, false)
	}
	locked = true

	existing, err := validateAndReadInstructionTarget(target, targetPath)
	if err != nil {
		return rollbackInstructionAppend(err, 0, false)
	}
	originalLength := int64(len(existing))
	appendBytes := instructionAppendBytes(existing, contents)
	if _, err := target.Seek(0, io.SeekEnd); err != nil {
		return rollbackInstructionAppend(fmt.Errorf("seek target %q for append: %w", targetPath, err), originalLength, false)
	}
	if err := writeAll(target, appendBytes); err != nil {
		return rollbackInstructionAppend(fmt.Errorf("append Instruction to target %q: %w", targetPath, err), originalLength, true)
	}
	if err := instructionFailure(invocation, instructionAfterWrite); err != nil {
		return rollbackInstructionAppend(fmt.Errorf("append Instruction to target %q: %w", targetPath, err), originalLength, true)
	}
	if err := instructionFailure(invocation, instructionBeforeSync); err != nil {
		return rollbackInstructionAppend(fmt.Errorf("synchronize target %q: %w", targetPath, err), originalLength, true)
	}
	if err := target.Sync(); err != nil {
		return rollbackInstructionAppend(fmt.Errorf("synchronize target %q: %w", targetPath, err), originalLength, true)
	}
	if err := releaseInstructionTargetLock(target); err != nil {
		locked = false
		return err
	}
	locked = false

	return nil
}

func parseTopLevelInstructionPath(path string) (string, error) {
	if path == "" || filepath.IsAbs(path) || strings.Contains(path, "/") || strings.Contains(path, `\`) || filepath.Ext(path) != ".md" {
		return "", fmt.Errorf("instruction Path %q must be a top-level lowercase kebab-case .md filename", path)
	}
	stem := strings.TrimSuffix(path, ".md")
	if !validInstructionName(stem) {
		return "", fmt.Errorf("instruction Path %q must be a top-level lowercase kebab-case .md filename", path)
	}

	return path, nil
}

func validInstructionName(name string) bool {
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

func readInstruction(path, instructionPath string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("inspect Instruction %q: %w", instructionPath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("instruction %q must be a non-symlinked regular file", instructionPath)
	}
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, fmt.Errorf("open Instruction %q: %w", instructionPath, err)
	}
	openedInfo, statErr := file.Stat()
	if statErr != nil {
		_ = file.Close()

		return nil, fmt.Errorf("inspect opened Instruction %q: %w", instructionPath, statErr)
	}
	if !openedInfo.Mode().IsRegular() {
		_ = file.Close()

		return nil, fmt.Errorf("instruction %q must be a non-symlinked regular file", instructionPath)
	}
	contents, readErr := io.ReadAll(file)
	closeErr := file.Close()
	if readErr != nil {
		return nil, fmt.Errorf("read Instruction %q: %w", instructionPath, readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close Instruction %q: %w", instructionPath, closeErr)
	}
	if !utf8.Valid(contents) {
		return nil, fmt.Errorf("instruction %q must contain valid UTF-8", instructionPath)
	}
	if bytes.HasPrefix(contents, []byte{0xef, 0xbb, 0xbf}) {
		return nil, fmt.Errorf("instruction %q must not begin with a UTF-8 BOM", instructionPath)
	}
	if strings.TrimSpace(string(contents)) == "" {
		return nil, fmt.Errorf("instruction %q must contain meaningful content", instructionPath)
	}

	return contents, nil
}

func openInstructionTarget(path string) (*os.File, bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		file, createErr := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0o644)
		if createErr != nil {
			return nil, false, fmt.Errorf("create target %q: %w", path, createErr)
		}

		return file, true, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("inspect target %q: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, false, fmt.Errorf("target %q must not be a symlink", path)
	}
	file, err := os.OpenFile(path, os.O_RDWR|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, false, fmt.Errorf("open target %q: %w", path, err)
	}

	return file, false, nil
}

func validateAndReadInstructionTarget(file *os.File, path string) ([]byte, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("inspect target %q: %w", path, err)
	}
	pathInfo, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("inspect locked target %q: %w", path, err)
	}
	if pathInfo.Mode()&os.ModeSymlink != 0 || !os.SameFile(info, pathInfo) {
		return nil, fmt.Errorf("target %q changed while waiting for its lock", path)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("target %q must be a regular file", path)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Nlink != 1 {
		return nil, fmt.Errorf("target %q must have exactly one filesystem link", path)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek target %q: %w", path, err)
	}
	contents, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read target %q: %w", path, err)
	}
	if !utf8.Valid(contents) {
		return nil, fmt.Errorf("target %q must contain valid UTF-8", path)
	}

	return contents, nil
}

func instructionAppendBytes(existing, instruction []byte) []byte {
	additionalLFs := 0
	if len(existing) > 0 {
		boundaryLFs := trailingLFs(existing) + leadingLFs(instruction)
		if boundaryLFs < 2 {
			additionalLFs = 2 - boundaryLFs
		}
	}
	finalLFs := 0
	if len(instruction) == 0 || instruction[len(instruction)-1] != '\n' {
		finalLFs = 1
	}
	result := make([]byte, 0, additionalLFs+len(instruction)+finalLFs)
	result = append(result, bytes.Repeat([]byte{'\n'}, additionalLFs)...)
	result = append(result, instruction...)
	result = append(result, bytes.Repeat([]byte{'\n'}, finalLFs)...)

	return result
}

func trailingLFs(contents []byte) int {
	count := 0
	for index := len(contents) - 1; index >= 0 && contents[index] == '\n'; index-- {
		count++
	}

	return count
}

func leadingLFs(contents []byte) int {
	count := 0
	for count < len(contents) && contents[count] == '\n' {
		count++
	}

	return count
}

func writeAll(file *os.File, contents []byte) error {
	for len(contents) > 0 {
		written, err := file.Write(contents)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
		contents = contents[written:]
	}

	return nil
}

func acquireInstructionTargetLock(ctx context.Context, file *os.File, path string, timeout time.Duration) error {
	if timeout <= 0 || timeout > defaultProjectLockTimeout {
		timeout = defaultProjectLockTimeout
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	retry := time.NewTicker(10 * time.Millisecond)
	defer retry.Stop()

	for {
		err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			return fmt.Errorf("lock target %q: %w", path, err)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("lock target %q: %w", path, ctx.Err())
		case <-deadline.C:
			return fmt.Errorf("target %q is locked by another Bond process", path)
		case <-retry.C:
		}
	}
}

func releaseInstructionTargetLock(file *os.File) error {
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_UN); err != nil {
		_ = file.Close()

		return fmt.Errorf("unlock target: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close target lock: %w", err)
	}

	return nil
}

func instructionFailure(invocation Invocation, point string) error {
	if invocation.transactionHook != nil {
		if err := invocation.transactionHook(point); err != nil {
			return err
		}
	}
	if invocation.transactionFailurePoint == point {
		return fmt.Errorf("injected failure at %s", point)
	}

	return nil
}
