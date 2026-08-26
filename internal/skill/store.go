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
	Paths       []string
	Diagnostics []string
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
