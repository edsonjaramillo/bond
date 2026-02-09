package skills

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Skill describes a globally available skill directory entry.
type Skill struct {
	Name string
	Path string
}

// Discover returns all valid skill directories in sourceDir sorted by name.
// A skill is valid only when it is a directory containing SKILL.md.
func Discover(sourceDir string) ([]Skill, error) {
	sourceAbs, err := filepath.Abs(sourceDir)
	if err != nil {
		return nil, err
	}

	skills := []Skill{}
	seen := map[string]string{}
	err = filepath.WalkDir(sourceAbs, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if errors.Is(walkErr, os.ErrNotExist) {
				return nil
			}
			return walkErr
		}
		if d.IsDir() || d.Name() != "SKILL.md" {
			return nil
		}

		skillDir := filepath.Dir(path)
		if filepath.Clean(skillDir) == filepath.Clean(sourceAbs) {
			return nil
		}

		name := filepath.Base(skillDir)
		if previous, exists := seen[name]; exists {
			return fmt.Errorf("duplicate skill %q found in %q and %q", name, previous, skillDir)
		}
		seen[name] = skillDir
		skills = append(skills, Skill{Name: name, Path: skillDir})
		return nil
	})
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Skill{}, nil
		}
		return nil, err
	}

	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
	return skills, nil
}
