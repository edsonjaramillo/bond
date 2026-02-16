package skills

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// ValidationIssue describes a single validation rule violation.
type ValidationIssue struct {
	Rule    string
	Message string
}

// ValidationResult captures validation issues for a single skill directory.
type ValidationResult struct {
	Name   string
	Path   string
	Issues []ValidationIssue
}

// ValidateStoreAll validates all discovered store skills in deterministic name order.
func ValidateStoreAll(storeDir string) ([]ValidationResult, error) {
	discovered, err := Discover(storeDir)
	if err != nil {
		return nil, err
	}

	results := make([]ValidationResult, 0, len(discovered))
	for _, skill := range discovered {
		result, err := ValidateSkillDir(skill.Path)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

// ValidateStoreByName validates a single skill by directory basename in storeDir.
func ValidateStoreByName(storeDir, name string) (ValidationResult, error) {
	path, err := findSkillDirByName(storeDir, name)
	if err != nil {
		return ValidationResult{}, err
	}
	return ValidateSkillDir(path)
}

// ValidateSkillDir validates one skill directory against specification-required checks.
func ValidateSkillDir(skillDir string) (ValidationResult, error) {
	skillAbs, err := filepath.Abs(skillDir)
	if err != nil {
		return ValidationResult{}, err
	}

	result := ValidationResult{
		Name: filepath.Base(skillAbs),
		Path: skillAbs,
	}

	marker := filepath.Join(skillAbs, "SKILL.md")
	info, err := os.Stat(marker)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			result.Issues = append(result.Issues, ValidationIssue{
				Rule:    "skill-file",
				Message: fmt.Sprintf("missing SKILL.md (expected at %q)", marker),
			})
			return result, nil
		}
		return ValidationResult{}, err
	}
	if info.IsDir() {
		result.Issues = append(result.Issues, ValidationIssue{
			Rule:    "skill-file",
			Message: fmt.Sprintf("SKILL.md must be a file, but %q is a directory", marker),
		})
		return result, nil
	}

	raw, err := os.ReadFile(marker)
	if err != nil {
		return ValidationResult{}, err
	}

	frontmatter, ok := extractFrontmatter(string(raw))
	if !ok {
		result.Issues = append(result.Issues, ValidationIssue{
			Rule:    "frontmatter",
			Message: "SKILL.md must begin with YAML frontmatter: open with '---' on line 1 and close with a separate '---' line",
		})
		return result, nil
	}

	meta := map[string]any{}
	if err := yaml.Unmarshal([]byte(frontmatter), &meta); err != nil {
		result.Issues = append(result.Issues, ValidationIssue{
			Rule:    "frontmatter",
			Message: fmt.Sprintf("invalid YAML in SKILL.md frontmatter: %v", err),
		})
		return result, nil
	}

	name, ok := requiredString(meta, "name")
	if !ok {
		result.Issues = append(result.Issues, ValidationIssue{
			Rule:    "name",
			Message: `frontmatter field "name" is required and must be a non-empty string`,
		})
	} else {
		nameCheck := CheckSkillName(name)
		if nameCheck.TooLong {
			result.Issues = append(result.Issues, ValidationIssue{
				Rule:    "name",
				Message: fmt.Sprintf(`frontmatter field "name" is %d characters; maximum is %d`, nameCheck.RuneCount, SkillNameMaxRunes),
			})
		}
		if nameCheck.InvalidFormat {
			result.Issues = append(result.Issues, ValidationIssue{
				Rule:    "name",
				Message: `frontmatter field "name" must use lowercase letters, numbers, and single hyphens only (for example: "go", "web-api")`,
			})
		}
		if filepath.Base(skillAbs) != name {
			result.Issues = append(result.Issues, ValidationIssue{
				Rule:    "name",
				Message: fmt.Sprintf(`frontmatter field "name" is %q, but the skill directory is %q; these must match`, name, filepath.Base(skillAbs)),
			})
		}
	}

	description, ok := requiredString(meta, "description")
	if !ok {
		result.Issues = append(result.Issues, ValidationIssue{
			Rule:    "description",
			Message: `frontmatter field "description" is required and must be a non-empty string`,
		})
	} else {
		descriptionLen := utf8.RuneCountInString(description)
		if descriptionLen > 1024 {
			result.Issues = append(result.Issues, ValidationIssue{
				Rule:    "description",
				Message: fmt.Sprintf(`frontmatter field "description" is %d characters; maximum is 1024`, descriptionLen),
			})
		}
	}

	return result, nil
}

func requiredString(meta map[string]any, field string) (string, bool) {
	value, ok := meta[field]
	if !ok {
		return "", false
	}
	str, ok := value.(string)
	if !ok {
		return "", false
	}
	str = strings.TrimSpace(str)
	if str == "" {
		return "", false
	}
	return str, true
}

func extractFrontmatter(contents string) (string, bool) {
	normalized := strings.ReplaceAll(strings.TrimPrefix(contents, "\uFEFF"), "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return "", false
	}

	rest := normalized[len("---\n"):]
	closing := strings.Index(rest, "\n---\n")
	if closing >= 0 {
		return rest[:closing], true
	}
	if strings.HasSuffix(rest, "\n---") {
		return strings.TrimSuffix(rest, "\n---"), true
	}
	if rest == "---" {
		return "", true
	}

	return "", false
}

func findSkillDirByName(storeDir, name string) (string, error) {
	storeAbs, err := filepath.Abs(storeDir)
	if err != nil {
		return "", err
	}

	var matches []string
	walkErr := filepath.WalkDir(storeAbs, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if errors.Is(walkErr, os.ErrNotExist) {
				return nil
			}
			return walkErr
		}
		if !d.IsDir() {
			return nil
		}
		if filepath.Clean(path) == filepath.Clean(storeAbs) {
			return nil
		}
		if filepath.Base(path) == name {
			matches = append(matches, path)
		}
		return nil
	})
	if walkErr != nil {
		return "", walkErr
	}

	sort.Strings(matches)
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("skill %q not found in store directory %q", name, storeAbs)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("skill %q is ambiguous in store directory %q", name, storeAbs)
	}
}
