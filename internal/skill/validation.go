package skill

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

const maxSymlinkExpansions = 255

// Validate reports every validation error found in a Skill directory.
func Validate(directory string) []error {
	var validationErrors []error
	documentPath := filepath.Join(directory, "SKILL.md")
	documentInfo, err := os.Lstat(documentPath)
	if err != nil {
		if os.IsNotExist(err) {
			validationErrors = append(validationErrors, fmt.Errorf("SKILL.md is missing"))
		} else {
			validationErrors = append(validationErrors, fmt.Errorf("inspect SKILL.md: %w", err))
		}
	} else if documentInfo.Mode()&os.ModeSymlink != 0 {
		validationErrors = append(validationErrors, fmt.Errorf("SKILL.md must be a regular file, not a symlink"))
	} else if !documentInfo.Mode().IsRegular() {
		validationErrors = append(validationErrors, fmt.Errorf("SKILL.md must be a regular file"))
	} else {
		validationErrors = append(validationErrors, validateDocument(documentPath, filepath.Base(directory))...)
	}

	validationErrors = append(validationErrors, validateTree(directory)...)

	return validationErrors
}

func validateDocument(path, directoryName string) []error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return []error{fmt.Errorf("read SKILL.md: %w", err)}
	}
	if !utf8.Valid(contents) {
		return []error{fmt.Errorf("SKILL.md must contain valid UTF-8")}
	}

	frontmatter, err := extractFrontmatter(contents)
	if err != nil {
		return []error{err}
	}

	var decoded any
	if err := yaml.Unmarshal(frontmatter, &decoded); err != nil {
		return []error{fmt.Errorf("SKILL.md frontmatter is invalid YAML: %w", err)}
	}

	var document yaml.Node
	if err := yaml.Unmarshal(frontmatter, &document); err != nil {
		return []error{fmt.Errorf("SKILL.md frontmatter is invalid YAML: %w", err)}
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return []error{fmt.Errorf("SKILL.md frontmatter must be a YAML mapping")}
	}

	mapping := document.Content[0]
	var validationErrors []error
	name, nameErrors := requiredString(mapping, "name")
	validationErrors = append(validationErrors, nameErrors...)
	_, descriptionErrors := requiredString(mapping, "description")
	validationErrors = append(validationErrors, descriptionErrors...)
	if len(nameErrors) == 0 {
		trimmedName := strings.TrimSpace(name)
		if !validName(trimmedName) {
			validationErrors = append(validationErrors, fmt.Errorf("skill name %q must use lowercase kebab-case", trimmedName))
		} else if trimmedName != directoryName {
			validationErrors = append(validationErrors, fmt.Errorf("skill name %q must match directory name %q", trimmedName, directoryName))
		}
	}

	return validationErrors
}

func extractFrontmatter(contents []byte) ([]byte, error) {
	lines := bytes.Split(contents, []byte{'\n'})
	if len(lines) == 0 || string(bytes.TrimSuffix(lines[0], []byte{'\r'})) != "---" {
		return nil, fmt.Errorf("SKILL.md must begin with YAML frontmatter")
	}
	for index := 1; index < len(lines); index++ {
		if string(bytes.TrimSuffix(lines[index], []byte{'\r'})) == "---" {
			return bytes.Join(lines[1:index], []byte{'\n'}), nil
		}
	}

	return nil, fmt.Errorf("SKILL.md frontmatter is missing its closing delimiter")
}

func requiredString(mapping *yaml.Node, field string) (string, []error) {
	var values []*yaml.Node
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		key := mapping.Content[index]
		if key.Kind == yaml.ScalarNode && key.Value == field {
			values = append(values, mapping.Content[index+1])
		}
	}
	if len(values) == 0 {
		return "", []error{fmt.Errorf("SKILL.md frontmatter field %q is required", field)}
	}
	if len(values) > 1 {
		return "", []error{fmt.Errorf("SKILL.md frontmatter field %q must not be repeated", field)}
	}

	value := resolvedYAMLNode(values[0])
	if value.Kind != yaml.ScalarNode || value.ShortTag() != "!!str" {
		return "", []error{fmt.Errorf("SKILL.md frontmatter field %q must be a string", field)}
	}
	if strings.TrimSpace(value.Value) == "" {
		return "", []error{fmt.Errorf("SKILL.md frontmatter field %q must not be empty", field)}
	}

	return value.Value, nil
}

func resolvedYAMLNode(node *yaml.Node) *yaml.Node {
	for node.Kind == yaml.AliasNode && node.Alias != nil {
		node = node.Alias
	}

	return node
}

func validateTree(directory string) []error {
	var validationErrors []error
	err := filepath.WalkDir(directory, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			validationErrors = append(validationErrors, fmt.Errorf("inspect resource %q: %w", resourcePath(directory, path), walkErr))

			return nil
		}
		if path == directory {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("inspect resource %q: %w", resourcePath(directory, path), err))

			return nil
		}
		mode := info.Mode()
		switch {
		case mode.IsDir(), mode.IsRegular():
			return nil
		case mode&os.ModeSymlink != 0:
			if filepath.Base(path) == "SKILL.md" && filepath.Dir(path) == directory {
				return nil
			}
			if err := validateResourceSymlink(directory, path); err != nil {
				validationErrors = append(validationErrors, fmt.Errorf("resource %q: %w", resourcePath(directory, path), err))
			}

			return nil
		default:
			validationErrors = append(validationErrors, fmt.Errorf("resource %q must be a regular file, directory, or safe relative symlink", resourcePath(directory, path)))

			return nil
		}
	})
	if err != nil {
		validationErrors = append(validationErrors, fmt.Errorf("inspect Skill resources: %w", err))
	}

	return validationErrors
}

func resourcePath(directory, path string) string {
	relativePath, err := filepath.Rel(directory, path)
	if err != nil {
		return path
	}

	return filepath.ToSlash(relativePath)
}

func validateResourceSymlink(root, link string) error {
	target, err := os.Readlink(link)
	if err != nil {
		return fmt.Errorf("read symlink: %w", err)
	}
	if filepath.IsAbs(target) {
		return fmt.Errorf("symlink target must be relative")
	}

	root, err = filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve Skill directory: %w", err)
	}
	link, err = filepath.Abs(link)
	if err != nil {
		return fmt.Errorf("resolve symlink: %w", err)
	}

	return resolveWithinRoot(root, link)
}

func resolveWithinRoot(root, path string) error {
	relativePath, err := filepath.Rel(root, path)
	if err != nil || escapesRoot(relativePath) {
		return fmt.Errorf("symlink escapes the Skill directory")
	}

	remaining := splitPath(relativePath)
	resolved := root
	seenStates := make(map[string]bool)
	symlinkExpansions := 0
	for len(remaining) > 0 {
		component := remaining[0]
		remaining = remaining[1:]
		switch component {
		case "", ".":
			continue
		case "..":
			if resolved == root {
				return fmt.Errorf("symlink escapes the Skill directory")
			}
			resolved = filepath.Dir(resolved)

			continue
		}

		candidate := filepath.Join(resolved, component)
		info, err := os.Lstat(candidate)
		if err != nil {
			return fmt.Errorf("symlink target is broken: %w", err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			if len(remaining) > 0 && !info.IsDir() {
				return fmt.Errorf("symlink target is broken")
			}
			resolved = candidate

			continue
		}

		symlinkExpansions++
		if symlinkExpansions > maxSymlinkExpansions {
			return fmt.Errorf("symlink target is cyclic")
		}
		state := candidate + "\x00" + strings.Join(remaining, "\x00")
		if seenStates[state] {
			return fmt.Errorf("symlink target is cyclic")
		}
		seenStates[state] = true

		target, err := os.Readlink(candidate)
		if err != nil {
			return fmt.Errorf("read symlink target: %w", err)
		}
		if filepath.IsAbs(target) {
			return fmt.Errorf("symlink target must be relative")
		}
		remaining = append(splitPath(target), remaining...)
	}

	return nil
}

func escapesRoot(relativePath string) bool {
	return relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) || filepath.IsAbs(relativePath)
}

func splitPath(path string) []string {
	if path == "" {
		return nil
	}

	return strings.Split(path, string(filepath.Separator))
}
