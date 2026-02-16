package skills

import (
	"errors"
	"os"
	"path/filepath"
)

// hasSkillMarker reports whether dir contains a file named SKILL.md.
func hasSkillMarker(dir string) (bool, error) {
	markerPath := filepath.Join(dir, "SKILL.md")
	markerInfo, err := os.Stat(markerPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if markerInfo.IsDir() {
		return false, nil
	}
	return true, nil
}
