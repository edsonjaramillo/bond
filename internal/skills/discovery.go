package skills

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
)

// Skill describes a globally available skill directory entry.
type Skill struct {
	Name string
	Path string
}

// Discover returns all entries in sourceDir sorted by name.
func Discover(sourceDir string) ([]Skill, error) {
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Skill{}, nil
		}
		return nil, err
	}

	skills := make([]Skill, 0, len(entries))
	for _, entry := range entries {
		skills = append(skills, Skill{
			Name: entry.Name(),
			Path: filepath.Join(sourceDir, entry.Name()),
		})
	}

	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Name < skills[j].Name
	})

	return skills, nil
}
