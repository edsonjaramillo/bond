package application

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/edsonjaramillo/bond/internal/skill"
)

const manifestVersion = 1

type projectManifest struct {
	Version int                  `json:"version"`
	Skills  []managedSkillRecord `json:"skills"`
}

type installationMode string

const (
	linkMode installationMode = "link"
	copyMode installationMode = "copy"
)

type managedSkillRecord struct {
	Name        string           `json:"name"`
	Source      string           `json:"source"`
	Mode        installationMode `json:"mode"`
	Destination string           `json:"destination"`
}

func emptyProjectManifest() projectManifest {
	return projectManifest{Version: manifestVersion, Skills: []managedSkillRecord{}}
}

func readProjectManifest(agentsDirectory string) (projectManifest, error) {
	path := filepath.Join(agentsDirectory, "bond-manifest.json")
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return emptyProjectManifest(), nil
	}
	if err != nil {
		return projectManifest{}, fmt.Errorf("inspect Project manifest: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return projectManifest{}, fmt.Errorf("path for Project infrastructure .agents/bond-manifest.json must not be a symlink")
	}
	if !info.Mode().IsRegular() {
		return projectManifest{}, fmt.Errorf("path for Project infrastructure .agents/bond-manifest.json must be a regular file")
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		return projectManifest{}, fmt.Errorf("read Project manifest: %w", err)
	}
	var manifest projectManifest
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return projectManifest{}, fmt.Errorf("project manifest is corrupt: %w", err)
	}
	if err := ensureJSONEnd(decoder); err != nil {
		return projectManifest{}, fmt.Errorf("project manifest is corrupt: %w", err)
	}
	if manifest.Version != manifestVersion {
		return projectManifest{}, fmt.Errorf("project manifest version %d is unsupported", manifest.Version)
	}
	if manifest.Skills == nil {
		return projectManifest{}, fmt.Errorf("project manifest is corrupt: skills must be an array")
	}
	if err := validateManifestRecords(manifest.Skills); err != nil {
		return projectManifest{}, fmt.Errorf("project manifest is corrupt: %w", err)
	}

	return manifest, nil
}

func ensureJSONEnd(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}

		return err
	}

	return nil
}

func validateManifestRecords(records []managedSkillRecord) error {
	names := make(map[string]bool)
	destinations := make(map[string]bool)
	for index, record := range records {
		storedPath, err := skill.ParseStoredPath(record.Source)
		if err != nil {
			return fmt.Errorf("skill record %d has invalid source: %w", index, err)
		}
		if record.Name != storedPath.Name {
			return fmt.Errorf("skill record %d name %q does not match source Skill Name %q", index, record.Name, storedPath.Name)
		}
		if record.Mode != linkMode && record.Mode != copyMode {
			return fmt.Errorf("skill record %d has unsupported mode %q", index, record.Mode)
		}
		wantDestination := filepath.ToSlash(filepath.Join(".agents", "skills", record.Name))
		if record.Destination != wantDestination {
			return fmt.Errorf("skill record %d has invalid destination %q", index, record.Destination)
		}
		if names[record.Name] {
			return fmt.Errorf("recorded Skill Name %q is repeated", record.Name)
		}
		if destinations[record.Destination] {
			return fmt.Errorf("destination %q is repeated", record.Destination)
		}
		names[record.Name] = true
		destinations[record.Destination] = true
	}

	return nil
}

func writeProjectManifest(agentsDirectory string, manifest projectManifest) error {
	contents, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode Project manifest: %w", err)
	}
	contents = append(contents, '\n')

	if err := writeAtomicFile(agentsDirectory, "bond-manifest.json", ".bond-manifest-*", contents); err != nil {
		return fmt.Errorf("replace Project manifest: %w", err)
	}

	return nil
}

func removeTemporaryManifest(path string, cause error) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("%w; also failed to remove temporary Project manifest: %v", cause, err)
	}

	return cause
}
