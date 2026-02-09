package skills

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
)

// DiscoverProjectLinkedGlobal returns project symlink entries that point into globalSkillsDir.
func DiscoverProjectLinkedGlobal(projectSkillsDir, globalSkillsDir string) ([]Entry, error) {
	projectAbs, err := filepath.Abs(projectSkillsDir)
	if err != nil {
		return nil, err
	}
	globalAbs, err := filepath.Abs(globalSkillsDir)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(projectAbs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Entry{}, nil
		}
		return nil, err
	}

	linked := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink == 0 {
			continue
		}

		entryPath := filepath.Join(projectAbs, entry.Name())
		target, err := os.Readlink(entryPath)
		if err != nil {
			return nil, err
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(entryPath), target)
		}

		targetAbs, err := filepath.Abs(target)
		if err != nil {
			return nil, err
		}

		if !isWithinDir(targetAbs, globalAbs) {
			continue
		}

		linked = append(linked, Entry{
			Name: entry.Name(),
			Path: entryPath,
		})
	}

	sort.Slice(linked, func(i, j int) bool {
		return linked[i].Name < linked[j].Name
	})

	return linked, nil
}
