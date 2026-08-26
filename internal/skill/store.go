// Package skill validates and discovers Bond Skills.
package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// StoreDiscovery is the observable result of inspecting a Store.
type StoreDiscovery struct {
	Paths         []string
	Organizations []string
	Diagnostics   []string
}

// ProjectDiscovery is the observable result of inspecting a Project Skill collection.
type ProjectDiscovery struct {
	Names       []string
	Diagnostics []string
}

// DiscoverProject inspects the immediate entries in a Project Skill collection.
func DiscoverProject(collection string) (ProjectDiscovery, error) {
	entries, err := os.ReadDir(collection)
	if err != nil {
		return ProjectDiscovery{}, err
	}

	var discovery ProjectDiscovery
	for _, entry := range entries {
		name := entry.Name()
		if entry.Type()&os.ModeSymlink == 0 && !entry.IsDir() {
			discovery.Diagnostics = append(discovery.Diagnostics, fmt.Sprintf("%s: unexpected Project entry; expected a Skill directory", name))

			continue
		}

		errors := ValidateProjectSkill(filepath.Join(collection, name), name)
		if len(errors) == 0 {
			discovery.Names = append(discovery.Names, name)

			continue
		}
		for _, err := range errors {
			discovery.Diagnostics = append(discovery.Diagnostics, fmt.Sprintf("%s: %v", name, err))
		}
	}
	sort.Strings(discovery.Names)

	return discovery, nil
}

// ValidateProjectSkill validates one Project Skill directory or directory symlink.
func ValidateProjectSkill(path, name string) []error {
	validationPath, err := ResolveProjectSkillDirectory(path)
	if err != nil {
		return []error{err}
	}

	errors := Validate(validationPath)
	if filepath.Base(validationPath) != name && len(errors) == 0 {
		errors = append(errors, fmt.Errorf("skill directory name %q must match Project directory basename %q", filepath.Base(validationPath), name))
	}

	return errors
}

// ResolveProjectSkillDirectory resolves a Project Skill entry without validating its contents.
func ResolveProjectSkillDirectory(path string) (string, error) {
	validationPath := path
	info, err := os.Lstat(path)
	if err != nil {
		return "", fmt.Errorf("inspect Project Skill: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		validationPath, err = filepath.EvalSymlinks(path)
		if err != nil {
			return "", fmt.Errorf("resolve Project Skill symlink: %w", err)
		}
		info, err = os.Stat(validationPath)
		if err != nil {
			return "", fmt.Errorf("inspect Project Skill target: %w", err)
		}
	}
	if !info.IsDir() {
		return "", fmt.Errorf("entry for Project Skill must be a directory")
	}

	return validationPath, nil
}

// DiscoverStore inspects top-level Stored Skills and Skills one Organization deep.
func DiscoverStore(store string) (StoreDiscovery, error) {
	entries, err := os.ReadDir(store)
	if err != nil {
		return StoreDiscovery{}, err
	}

	var discovery StoreDiscovery
	for _, entry := range entries {
		discoverRootEntry(store, entry, &discovery)
	}
	sort.Strings(discovery.Paths)
	sort.Strings(discovery.Organizations)

	return discovery, nil
}

func discoverRootEntry(store string, entry os.DirEntry, discovery *StoreDiscovery) {
	name := entry.Name()
	path := filepath.Join(store, name)
	if entry.Type()&os.ModeSymlink != 0 {
		discovery.Diagnostics = append(discovery.Diagnostics, fmt.Sprintf("%s: Store entry must not be a symlink", name))

		return
	}
	if !entry.IsDir() {
		discovery.Diagnostics = append(discovery.Diagnostics, fmt.Sprintf("%s: unexpected Store entry; expected a Skill or Organization directory", name))

		return
	}

	_, skillDocumentError := os.Lstat(filepath.Join(path, "SKILL.md"))
	if skillDocumentError == nil {
		discoverSkill(path, filepath.ToSlash(name), discovery)

		return
	}
	if !os.IsNotExist(skillDocumentError) {
		discovery.Diagnostics = append(discovery.Diagnostics, fmt.Sprintf("%s: inspect SKILL.md: %v", name, skillDocumentError))

		return
	}

	discoverOrganization(path, name, discovery)
}

func discoverOrganization(path, name string, discovery *StoreDiscovery) {
	organizationValid := validName(name)
	if !organizationValid {
		discovery.Diagnostics = append(discovery.Diagnostics, fmt.Sprintf("%s: Organization name must use lowercase kebab-case", name))
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		discovery.Diagnostics = append(discovery.Diagnostics, fmt.Sprintf("%s: read Organization: %v", name, err))

		return
	}
	if organizationValid {
		discovery.Organizations = append(discovery.Organizations, name)
	}
	for _, entry := range entries {
		relativePath := filepath.ToSlash(filepath.Join(name, entry.Name()))
		entryPath := filepath.Join(path, entry.Name())
		if entry.Type()&os.ModeSymlink != 0 {
			discovery.Diagnostics = append(discovery.Diagnostics, fmt.Sprintf("%s: Stored Skill entry must not be a symlink", relativePath))

			continue
		}
		if !entry.IsDir() {
			discovery.Diagnostics = append(discovery.Diagnostics, fmt.Sprintf("%s: unexpected Organization entry; expected a Skill directory", relativePath))

			continue
		}

		before := len(discovery.Diagnostics)
		discoverSkill(entryPath, relativePath, discovery)
		if !organizationValid && len(discovery.Diagnostics) == before {
			// A Skill beneath a malformed Organization is validated for diagnostics,
			// but cannot have a valid Store-relative identity.
			discovery.Diagnostics = append(discovery.Diagnostics, fmt.Sprintf("%s: parent Organization name is invalid", relativePath))
			removePath(&discovery.Paths, relativePath)
		}
	}
}

func discoverSkill(path, relativePath string, discovery *StoreDiscovery) {
	errors := Validate(path)
	if len(errors) == 0 {
		discovery.Paths = append(discovery.Paths, relativePath)

		return
	}
	for _, err := range errors {
		discovery.Diagnostics = append(discovery.Diagnostics, fmt.Sprintf("%s: %v", relativePath, err))
	}
}

func removePath(paths *[]string, path string) {
	for index, candidate := range *paths {
		if candidate == path {
			*paths = append((*paths)[:index], (*paths)[index+1:]...)

			return
		}
	}
}

func validName(name string) bool {
	if name == "" || strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") || strings.Contains(name, "--") {
		return false
	}
	for _, character := range name {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' {
			return false
		}
	}

	return true
}
