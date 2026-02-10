package skills

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
)

// DiscoverProjectStorable returns project-local skills that can be stored globally.
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

		markerPath := filepath.Join(skillPath, "SKILL.md")
		markerInfo, err := os.Stat(markerPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		if markerInfo.IsDir() {
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
