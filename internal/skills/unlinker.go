package skills

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
)

// Entry represents a project-local skill entry.
type Entry struct {
	Name string
	Path string
}

// DiscoverLinked returns only symlink entries in projectSkillsDir, sorted by name.
func DiscoverLinked(projectSkillsDir string) ([]Entry, error) {
	entries, err := os.ReadDir(projectSkillsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Treat a missing skills directory as "no linked entries".
			return []Entry{}, nil
		}
		return nil, err
	}

	linked := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink == 0 {
			continue
		}
		linked = append(linked, Entry{
			Name: entry.Name(),
			Path: filepath.Join(projectSkillsDir, entry.Name()),
		})
	}

	sort.Slice(linked, func(i, j int) bool {
		return linked[i].Name < linked[j].Name
	})

	return linked, nil
}

// Unlink removes path only when it exists and is a symlink.
func Unlink(path string) (bool, error) {
	// Lstat keeps symlink metadata instead of following the link.
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		// Refuse to delete regular files/directories.
		return false, nil
	}

	if err := os.Remove(path); err != nil {
		return false, err
	}
	return true, nil
}
