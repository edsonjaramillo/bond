package skills

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// LinkStatus describes the outcome of a link attempt.
type LinkStatus string

const (
	LinkStatusLinked        LinkStatus = "linked"
	LinkStatusAlreadyLinked LinkStatus = "already_linked"
	LinkStatusConflict      LinkStatus = "conflict"
)

// LinkResult wraps the final status for a link operation.
type LinkResult struct {
	Status LinkStatus
}

// Link creates destPath as a symlink to sourcePath if possible.
func Link(sourcePath, destPath string) (LinkResult, error) {
	// Canonicalize the source first so comparisons are stable regardless of cwd.
	sourceAbs, err := filepath.Abs(sourcePath)
	if err != nil {
		return LinkResult{}, err
	}

	if _, err := os.Lstat(sourceAbs); err != nil {
		return LinkResult{}, fmt.Errorf("source missing %q: %w", sourceAbs, err)
	}

	info, err := os.Lstat(destPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return LinkResult{}, err
		}
		// Destination does not exist yet; create the link directly.
		if err := os.Symlink(sourceAbs, destPath); err != nil {
			return LinkResult{}, err
		}
		return LinkResult{Status: LinkStatusLinked}, nil
	}

	// An existing non-symlink entry cannot be overwritten by this command.
	if info.Mode()&os.ModeSymlink == 0 {
		return LinkResult{Status: LinkStatusConflict}, nil
	}

	target, err := os.Readlink(destPath)
	if err != nil {
		return LinkResult{}, err
	}
	// Resolve relative symlink targets against the symlink's parent directory
	// before comparing with the requested source path.
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(destPath), target)
	}

	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return LinkResult{}, err
	}

	if filepath.Clean(targetAbs) == filepath.Clean(sourceAbs) {
		// Existing link already points to the same target.
		return LinkResult{Status: LinkStatusAlreadyLinked}, nil
	}

	return LinkResult{Status: LinkStatusConflict}, nil
}
