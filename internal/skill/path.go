package skill

import (
	"fmt"
	"strings"
)

// StoredPath identifies a top-level Stored Skill or one beneath an Organization.
type StoredPath struct {
	Organization string
	Name         string
}

// ParseStoredPath validates and separates a Store-relative Skill path.
func ParseStoredPath(storedSkillPath string) (StoredPath, error) {
	components := strings.Split(storedSkillPath, "/")
	if len(components) < 1 || len(components) > 2 {
		return StoredPath{}, fmt.Errorf("path %q for a Stored Skill must be a Skill Name or Organization/Skill Name", storedSkillPath)
	}
	for _, component := range components {
		if component == "" || component == "." || component == ".." {
			return StoredPath{}, fmt.Errorf("path %q for a Stored Skill contains an unsafe component", storedSkillPath)
		}
		if !validName(component) {
			return StoredPath{}, fmt.Errorf("component %q of a Stored Skill path must use lowercase kebab-case", component)
		}
	}

	if len(components) == 1 {
		return StoredPath{Name: components[0]}, nil
	}

	return StoredPath{Organization: components[0], Name: components[1]}, nil
}
