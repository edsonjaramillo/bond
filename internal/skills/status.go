package skills

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// StatusKind describes the observed state of a project skill entry.
type StatusKind string

const (
	StatusLinked   StatusKind = "linked"
	StatusBroken   StatusKind = "broken"
	StatusExternal StatusKind = "external"
	StatusConflict StatusKind = "conflict"
)

// StatusEntry is a classified project-local skill entry.
type StatusEntry struct {
	Name   string
	Path   string
	Status StatusKind
}

// StatusReport captures the health of project-local skill entries.
type StatusReport struct {
	ProjectSkillsDir string
	GlobalSkillsDir  string
	Entries          []StatusEntry
}

// InspectStatus classifies project entries against the global skills directory.
func InspectStatus(globalSkillsDir, projectSkillsDir string) (StatusReport, error) {
	globalAbs, err := filepath.Abs(globalSkillsDir)
	if err != nil {
		return StatusReport{}, err
	}
	projectAbs, err := filepath.Abs(projectSkillsDir)
	if err != nil {
		return StatusReport{}, err
	}

	report := StatusReport{
		ProjectSkillsDir: projectAbs,
		GlobalSkillsDir:  globalAbs,
		Entries:          []StatusEntry{},
	}

	entries, err := os.ReadDir(projectAbs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return report, nil
		}
		return StatusReport{}, err
	}

	report.Entries = make([]StatusEntry, 0, len(entries))
	for _, entry := range entries {
		entryPath := filepath.Join(projectAbs, entry.Name())

		if entry.Type()&os.ModeSymlink == 0 {
			report.Entries = append(report.Entries, StatusEntry{
				Name:   entry.Name(),
				Path:   entryPath,
				Status: StatusConflict,
			})
			continue
		}

		target, err := os.Readlink(entryPath)
		if err != nil {
			return StatusReport{}, err
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(entryPath), target)
		}

		targetAbs, err := filepath.Abs(target)
		if err != nil {
			return StatusReport{}, err
		}

		if _, err := os.Lstat(targetAbs); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				report.Entries = append(report.Entries, StatusEntry{
					Name:   entry.Name(),
					Path:   entryPath,
					Status: StatusBroken,
				})
				continue
			}
			return StatusReport{}, err
		}

		status := StatusExternal
		if isWithinDir(targetAbs, globalAbs) {
			status = StatusLinked
		}

		report.Entries = append(report.Entries, StatusEntry{
			Name:   entry.Name(),
			Path:   entryPath,
			Status: status,
		})
	}

	sort.Slice(report.Entries, func(i, j int) bool {
		return report.Entries[i].Name < report.Entries[j].Name
	})

	return report, nil
}

func isWithinDir(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	if rel == ".." {
		return false
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
