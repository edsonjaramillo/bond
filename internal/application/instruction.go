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
	"golang.org/x/sys/unix"
)

const (
	instructionAfterPreflight  = "instruction-after-preflight"
	instructionAfterParentOpen = "instruction-after-parent-open"
	instructionAfterWrite      = "instruction-after-write"
	instructionBeforeSync      = "instruction-before-sync"
)

func appendInstruction(command *cobra.Command, invocation Invocation, instructionPath, targetPath string) error {
	components, err := parseInstructionPath(instructionPath)
	if err != nil {
		return err
	}
	store, err := centralCollectionPath(invocation.Environment, instructionCollection)
	if err != nil {
		return err
	}
	resolvedStore, resolveErr := filepath.EvalSymlinks(store)
	if resolveErr == nil {
		store = resolvedStore
	} else if !os.IsNotExist(resolveErr) {
		return fmt.Errorf("resolve Instruction Store: %w", resolveErr)
	}
	contents, err := readInstruction(store, components, instructionPath)
	if err != nil {
		return err
	}

	projectRoot, projectPath, err := openInstructionProject(invocation.WorkingDirectory)
	if err != nil {
		return err
	}
	targetRelative, targetPath, err := validateInstructionTarget(projectPath, targetPath, store)
	if err != nil {
		_ = projectRoot.Close()

		return err
	}
	if err := instructionFailure(invocation, instructionAfterPreflight); err != nil {
		_ = projectRoot.Close()

		return fmt.Errorf("validate target %q: %w", targetPath, err)
	}
	target, parent, targetName, created, err := openInstructionTarget(projectRoot, targetRelative, targetPath, invocation)
	if err != nil {
		return err
	}
	locked := false
	rollbackInstructionAppend := func(operationErr error, originalLength int64, mutated bool) error {
		var rollbackErr error
		if created {
			// Unlink through the verified parent descriptor while the lock is held.
			// A cooperative waiter then rejects its stale target descriptor.
			if removeErr := removeCreatedInstructionTarget(target, parent, targetName, targetPath); removeErr != nil && !errors.Is(removeErr, unix.ENOENT) {
				rollbackErr = errors.Join(rollbackErr, removeErr)
			} else if removeErr == nil {
				rollbackErr = errors.Join(rollbackErr, parent.Sync())
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

		rollbackErr = errors.Join(rollbackErr, parent.Close())

		return errors.Join(operationErr, rollbackErr)
	}

	if err := acquireInstructionTargetLock(command.Context(), target, targetPath, invocation.projectLockTimeout); err != nil {
		return rollbackInstructionAppend(err, 0, false)
	}
	locked = true

	existing, err := validateAndReadInstructionTarget(target, parent, targetName, targetPath)
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
	if err := parent.Close(); err != nil {
		return fmt.Errorf("close target parent: %w", err)
	}

	return nil
}

func openInstructionProject(project string) (*os.File, string, error) {
	projectPath, err := filepath.EvalSymlinks(project)
	if err != nil {
		return nil, "", fmt.Errorf("resolve invocation directory: %w", err)
	}
	projectPath, err = filepath.Abs(projectPath)
	if err != nil {
		return nil, "", fmt.Errorf("resolve invocation directory: %w", err)
	}
	root, err := os.Open(projectPath)
	if err != nil {
		return nil, "", fmt.Errorf("open invocation directory: %w", err)
	}
	info, err := root.Stat()
	if err != nil {
		_ = root.Close()

		return nil, "", fmt.Errorf("inspect invocation directory: %w", err)
	}
	if !info.IsDir() {
		_ = root.Close()

		return nil, "", fmt.Errorf("invocation directory must be a directory")
	}

	return root, projectPath, nil
}

func validateInstructionTarget(projectPath, target, store string) (string, string, error) {
	if target == "" || filepath.IsAbs(target) {
		return "", "", fmt.Errorf("target %q must be a relative path beneath the invocation directory", target)
	}
	cleaned := filepath.Clean(target)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("target %q must be a relative path beneath the invocation directory", target)
	}
	portable := filepath.ToSlash(cleaned)
	foldedPortable := strings.ToLower(portable)
	if foldedPortable == ".agents" || strings.HasPrefix(foldedPortable, ".agents/") {
		return "", "", fmt.Errorf("target %q must not be beneath Bond project infrastructure", target)
	}
	targetPath := filepath.Join(projectPath, cleaned)
	storePath, err := filepath.Abs(store)
	if err != nil {
		return "", "", fmt.Errorf("resolve Instruction Store: %w", err)
	}
	if pathWithinFold(targetPath, storePath) {
		return "", "", fmt.Errorf("target %q must not be inside the Instruction Store", target)
	}

	// This ownership check intentionally does not acquire the Project mutation
	// lock. It fails closed for an existing invalid manifest, but remains
	// best-effort with respect to concurrent Resource mutation.
	manifest, err := readProjectManifest(filepath.Join(projectPath, ".agents"))
	if err != nil {
		return "", "", err
	}
	for _, resource := range manifest.Resources {
		for _, ownedPath := range resource.Paths {
			if pathsOverlapFold(portable, ownedPath) {
				return "", "", fmt.Errorf("target %q overlaps path %q owned by Managed Resource %q", target, ownedPath, resource.Name)
			}
		}
	}

	return cleaned, targetPath, nil
}

func pathWithinFold(path, root string) bool {
	relative, err := filepath.Rel(strings.ToLower(root), strings.ToLower(path))
	if err != nil {
		return false
	}
	folded := strings.ToLower(relative)

	return folded != ".." && !strings.HasPrefix(folded, ".."+string(filepath.Separator))
}

func pathsOverlapFold(first, second string) bool {
	return pathsOverlap(strings.ToLower(first), strings.ToLower(second))
}

func parseInstructionPath(path string) ([]string, error) {
	invalid := func() ([]string, error) {
		return nil, fmt.Errorf("invalid Instruction Path %q: must be name.md or group/name.md using lowercase kebab-case", path)
	}
	if path == "" || filepath.IsAbs(path) || strings.Contains(path, `\`) {
		return invalid()
	}
	components := strings.Split(path, "/")
	if len(components) < 1 || len(components) > 2 {
		return invalid()
	}
	filename := components[len(components)-1]
	if !strings.HasSuffix(filename, ".md") || !validInstructionName(strings.TrimSuffix(filename, ".md")) {
		return invalid()
	}
	if len(components) == 2 && !validInstructionName(components[0]) {
		return invalid()
	}

	return components, nil
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

func readInstruction(store string, components []string, instructionPath string) ([]byte, error) {
	rootFD, err := unix.Open(store, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("open Instruction Store for %q: %w", instructionPath, err)
	}
	root := os.NewFile(uintptr(rootFD), store)
	parent := root
	if len(components) == 2 {
		groupFD, openErr := unix.Openat(int(root.Fd()), components[0], unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if openErr != nil {
			_ = root.Close()

			return nil, fmt.Errorf("open Instruction grouping directory %q: %w", components[0], openErr)
		}
		group := os.NewFile(uintptr(groupFD), components[0])
		if _, matchErr := matchingInstructionSourceEntry(group, root, components[0], "grouping directory", instructionPath); matchErr != nil {
			_ = group.Close()
			_ = root.Close()

			return nil, matchErr
		}
		parent = group
		defer func() { _ = group.Close() }()
	}
	defer func() { _ = root.Close() }()

	filename := components[len(components)-1]
	fileFD, err := unix.Openat(int(parent.Fd()), filename, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("open Instruction %q: %w", instructionPath, err)
	}
	file := os.NewFile(uintptr(fileFD), instructionPath)
	opened, matchErr := matchingInstructionSourceEntry(file, parent, filename, "Instruction", instructionPath)
	if matchErr != nil {
		_ = file.Close()

		return nil, matchErr
	}
	if opened.Mode&unix.S_IFMT != unix.S_IFREG {
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

func matchingInstructionSourceEntry(file, parent *os.File, name, kind, instructionPath string) (unix.Stat_t, error) {
	var opened unix.Stat_t
	if err := unix.Fstat(int(file.Fd()), &opened); err != nil {
		return unix.Stat_t{}, fmt.Errorf("inspect opened %s %q: %w", kind, instructionPath, err)
	}
	var current unix.Stat_t
	if err := unix.Fstatat(int(parent.Fd()), name, &current, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return unix.Stat_t{}, fmt.Errorf("inspect %s %q: %w", kind, instructionPath, err)
	}
	if opened.Dev != current.Dev || opened.Ino != current.Ino || current.Mode&unix.S_IFMT == unix.S_IFLNK {
		return unix.Stat_t{}, fmt.Errorf("%s %q changed during validation", strings.ToLower(kind), instructionPath)
	}

	return opened, nil
}

func openInstructionTarget(root *os.File, relative, path string, invocation Invocation) (*os.File, *os.File, string, bool, error) {
	components := strings.Split(relative, string(filepath.Separator))
	directories := []*os.File{root}
	for _, component := range components[:len(components)-1] {
		parent := directories[len(directories)-1]
		fd, openErr := unix.Openat(int(parent.Fd()), component, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if openErr != nil {
			_ = closeInstructionDirectories(directories)

			return nil, nil, "", false, fmt.Errorf("open target parent for %q: %w", path, openErr)
		}
		directories = append(directories, os.NewFile(uintptr(fd), component))
	}
	if err := instructionFailure(invocation, instructionAfterParentOpen); err != nil {
		_ = closeInstructionDirectories(directories)

		return nil, nil, "", false, fmt.Errorf("validate target parent for %q: %w", path, err)
	}
	for index := 1; index < len(directories); index++ {
		if err := validateInstructionDirectoryEntry(directories[index-1], directories[index], components[index-1], path); err != nil {
			_ = closeInstructionDirectories(directories)

			return nil, nil, "", false, err
		}
	}

	parent := directories[len(directories)-1]
	name := components[len(components)-1]
	fd, err := unix.Openat(int(parent.Fd()), name, unix.O_RDWR|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	created := false
	if errors.Is(err, unix.ENOENT) {
		fd, err = unix.Openat(int(parent.Fd()), name, unix.O_RDWR|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0o644)
		created = err == nil
	}
	if err != nil {
		_ = closeInstructionDirectories(directories)

		return nil, nil, "", false, fmt.Errorf("open target %q: %w", path, err)
	}
	target := os.NewFile(uintptr(fd), path)
	for index := 1; index < len(directories); index++ {
		if validateErr := validateInstructionDirectoryEntry(directories[index-1], directories[index], components[index-1], path); validateErr != nil {
			if created {
				_ = removeCreatedInstructionTarget(target, parent, name, path)
			}
			_ = target.Close()
			_ = closeInstructionDirectories(directories)

			return nil, nil, "", false, validateErr
		}
	}
	for _, directory := range directories[:len(directories)-1] {
		if closeErr := directory.Close(); closeErr != nil {
			_ = target.Close()
			_ = parent.Close()

			return nil, nil, "", false, fmt.Errorf("close target ancestor: %w", closeErr)
		}
	}

	return target, parent, name, created, nil
}

func validateInstructionDirectoryEntry(parent, child *os.File, name, targetPath string) error {
	_, err := matchingInstructionEntry(child, parent, name, "target parent", targetPath)

	return err
}

func matchingInstructionEntry(file, parent *os.File, name, kind, targetPath string) (unix.Stat_t, error) {
	var opened unix.Stat_t
	if err := unix.Fstat(int(file.Fd()), &opened); err != nil {
		return unix.Stat_t{}, fmt.Errorf("inspect %s for %q: %w", kind, targetPath, err)
	}
	var current unix.Stat_t
	if err := unix.Fstatat(int(parent.Fd()), name, &current, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return unix.Stat_t{}, fmt.Errorf("inspect %s for %q: %w", kind, targetPath, err)
	}
	if opened.Dev != current.Dev || opened.Ino != current.Ino || current.Mode&unix.S_IFMT == unix.S_IFLNK {
		return unix.Stat_t{}, fmt.Errorf("%s for %q changed during validation", kind, targetPath)
	}

	return opened, nil
}

func removeCreatedInstructionTarget(file, parent *os.File, name, path string) error {
	if _, err := matchingInstructionEntry(file, parent, name, "newly created target", path); err != nil {
		return err
	}
	if err := unix.Unlinkat(int(parent.Fd()), name, 0); err != nil {
		return fmt.Errorf("remove newly created target %q: %w", path, err)
	}

	return nil
}

func closeInstructionDirectories(directories []*os.File) error {
	var result error
	for _, directory := range directories {
		result = errors.Join(result, directory.Close())
	}

	return result
}

func validateAndReadInstructionTarget(file, parent *os.File, name, path string) ([]byte, error) {
	opened, err := matchingInstructionEntry(file, parent, name, "locked target", path)
	if err != nil {
		return nil, err
	}
	if opened.Mode&unix.S_IFMT != unix.S_IFREG {
		return nil, fmt.Errorf("target %q must be a regular file", path)
	}
	if opened.Nlink != 1 {
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
