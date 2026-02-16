package skills

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
)

// DiscoverProjectAll returns all valid project-local skills directly under
// projectSkillsDir, including symlinked directories with a SKILL.md marker.
func DiscoverProjectAll(projectSkillsDir string) ([]Skill, error) {
	projectAbs, err := filepath.Abs(projectSkillsDir)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(projectAbs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Skill{}, nil
		}
		return nil, err
	}

	skills := make([]Skill, 0, len(entries))
	for _, entry := range entries {
		entryPath := filepath.Join(projectAbs, entry.Name())

		info, err := os.Lstat(entryPath)
		if err != nil {
			return nil, err
		}

		checkPath := entryPath
		if info.Mode()&os.ModeSymlink != 0 {
			resolved, err := filepath.EvalSymlinks(entryPath)
			if err != nil {
				continue
			}
			resolvedInfo, err := os.Stat(resolved)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				return nil, err
			}
			if !resolvedInfo.IsDir() {
				continue
			}
			checkPath = resolved
		} else if !info.IsDir() {
			continue
		}

		hasMarker, err := hasSkillMarker(checkPath)
		if err != nil {
			return nil, err
		}
		if !hasMarker {
			continue
		}

		skills = append(skills, Skill{
			Name: entry.Name(),
			Path: entryPath,
		})
	}

	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Name < skills[j].Name
	})

	return skills, nil
}

// DiscoverProjectStorable returns project-local skills that can be stored in the store directory.
// A storable skill is a non-symlink directory directly under projectSkillsDir
// containing a file named SKILL.md.
func DiscoverProjectStorable(projectSkillsDir string) ([]Skill, error) {
	projectAbs, err := filepath.Abs(projectSkillsDir)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(projectAbs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Skill{}, nil
		}
		return nil, err
	}

	skills := make([]Skill, 0, len(entries))
	for _, entry := range entries {
		skillPath := filepath.Join(projectAbs, entry.Name())

		info, err := os.Lstat(skillPath)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}

		hasMarker, err := hasSkillMarker(skillPath)
		if err != nil {
			return nil, err
		}
		if !hasMarker {
			continue
		}

		skills = append(skills, Skill{
			Name: entry.Name(),
			Path: skillPath,
		})
	}

	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Name < skills[j].Name
	})

	return skills, nil
}
