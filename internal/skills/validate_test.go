package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateSkillDirValid(t *testing.T) {
	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, "go")
	mustMkdirAllValidate(t, skillDir)
	mustWriteFileValidate(t, filepath.Join(skillDir, "SKILL.md"), "---\nname: go\ndescription: Go skill\n---\n# Go\n")

	result, err := ValidateSkillDir(skillDir)
	if err != nil {
		t.Fatalf("ValidateSkillDir() error = %v", err)
	}
	if len(result.Issues) != 0 {
		t.Fatalf("len(result.Issues) = %d, want 0", len(result.Issues))
	}
}

func TestValidateSkillDirMissingSkillFile(t *testing.T) {
	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, "go")
	mustMkdirAllValidate(t, skillDir)

	result, err := ValidateSkillDir(skillDir)
	if err != nil {
		t.Fatalf("ValidateSkillDir() error = %v", err)
	}
	if len(result.Issues) != 1 {
		t.Fatalf("len(result.Issues) = %d, want 1", len(result.Issues))
	}
	if result.Issues[0].Rule != "skill-file" {
		t.Fatalf("result.Issues[0].Rule = %q, want skill-file", result.Issues[0].Rule)
	}
}

func TestValidateSkillDirRequiresFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, "go")
	mustMkdirAllValidate(t, skillDir)
	mustWriteFileValidate(t, filepath.Join(skillDir, "SKILL.md"), "# Missing frontmatter\n")

	result, err := ValidateSkillDir(skillDir)
	if err != nil {
		t.Fatalf("ValidateSkillDir() error = %v", err)
	}
	if len(result.Issues) != 1 {
		t.Fatalf("len(result.Issues) = %d, want 1", len(result.Issues))
	}
	if result.Issues[0].Rule != "frontmatter" {
		t.Fatalf("result.Issues[0].Rule = %q, want frontmatter", result.Issues[0].Rule)
	}
}

func TestValidateSkillDirChecksRequiredFieldsAndNameRules(t *testing.T) {
	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, "go")
	mustMkdirAllValidate(t, skillDir)
	mustWriteFileValidate(t, filepath.Join(skillDir, "SKILL.md"), "---\nname: Go--Skill\ndescription: \"\"\n---\n")

	result, err := ValidateSkillDir(skillDir)
	if err != nil {
		t.Fatalf("ValidateSkillDir() error = %v", err)
	}
	if len(result.Issues) < 3 {
		t.Fatalf("len(result.Issues) = %d, want at least 3", len(result.Issues))
	}

	joined := joinIssues(result.Issues)
	if !strings.Contains(joined, `frontmatter field "name" must use lowercase letters, numbers, and single hyphens only`) {
		t.Fatalf("issues missing name format error: %q", joined)
	}
	if !strings.Contains(joined, `frontmatter field "name" is "Go--Skill", but the skill directory is "go"; these must match`) {
		t.Fatalf("issues missing name/directory mismatch: %q", joined)
	}
	if !strings.Contains(joined, `frontmatter field "description" is required and must be a non-empty string`) {
		t.Fatalf("issues missing description requirement: %q", joined)
	}
}

func TestValidateSkillDirLengthBounds(t *testing.T) {
	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, "go")
	mustMkdirAllValidate(t, skillDir)

	longName := strings.Repeat("a", 65)
	longDescription := strings.Repeat("d", 1025)
	mustWriteFileValidate(t, filepath.Join(skillDir, "SKILL.md"), "---\nname: "+longName+"\ndescription: "+longDescription+"\n---\n")

	result, err := ValidateSkillDir(skillDir)
	if err != nil {
		t.Fatalf("ValidateSkillDir() error = %v", err)
	}
	joined := joinIssues(result.Issues)
	if !strings.Contains(joined, `frontmatter field "name" is 65 characters; maximum is 64`) {
		t.Fatalf("issues missing name length error: %q", joined)
	}
	if !strings.Contains(joined, `frontmatter field "description" is 1025 characters; maximum is 1024`) {
		t.Fatalf("issues missing description length error: %q", joined)
	}
}

func TestValidateStoreByNameFindsNestedDir(t *testing.T) {
	tmp := t.TempDir()
	globalDir := filepath.Join(tmp, "global")
	skillDir := filepath.Join(globalDir, "lang", "go")
	mustMkdirAllValidate(t, skillDir)
	mustWriteFileValidate(t, filepath.Join(skillDir, "SKILL.md"), "---\nname: go\ndescription: Go skill\n---\n")

	result, err := ValidateStoreByName(globalDir, "go")
	if err != nil {
		t.Fatalf("ValidateStoreByName() error = %v", err)
	}
	if result.Name != "go" {
		t.Fatalf("result.Name = %q, want go", result.Name)
	}
}

func TestValidateStoreByNameAmbiguousReturnsError(t *testing.T) {
	tmp := t.TempDir()
	globalDir := filepath.Join(tmp, "global")
	mustMkdirAllValidate(t, filepath.Join(globalDir, "team-a", "go"))
	mustMkdirAllValidate(t, filepath.Join(globalDir, "team-b", "go"))

	_, err := ValidateStoreByName(globalDir, "go")
	if err == nil {
		t.Fatal("ValidateStoreByName() error = nil, want ambiguity error")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("ValidateStoreByName() error = %q, want ambiguity message", err)
	}
}

func TestValidateStoreAllValidatesDiscoveredSkills(t *testing.T) {
	tmp := t.TempDir()
	globalDir := filepath.Join(tmp, "global")
	goDir := filepath.Join(globalDir, "lang", "go")
	rustDir := filepath.Join(globalDir, "systems", "rust")

	mustMkdirAllValidate(t, goDir)
	mustMkdirAllValidate(t, rustDir)
	mustWriteFileValidate(t, filepath.Join(goDir, "SKILL.md"), "---\nname: go\ndescription: Go skill\n---\n")
	mustWriteFileValidate(t, filepath.Join(rustDir, "SKILL.md"), "---\nname: rust\ndescription: \"\"\n---\n")

	results, err := ValidateStoreAll(globalDir)
	if err != nil {
		t.Fatalf("ValidateStoreAll() error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if results[0].Name != "go" || results[1].Name != "rust" {
		t.Fatalf("result order = [%q, %q], want [go, rust]", results[0].Name, results[1].Name)
	}
	if len(results[0].Issues) != 0 {
		t.Fatalf("go skill issues = %d, want 0", len(results[0].Issues))
	}
	if len(results[1].Issues) == 0 {
		t.Fatalf("rust skill issues = %d, want >0", len(results[1].Issues))
	}
}

func joinIssues(issues []ValidationIssue) string {
	parts := make([]string, 0, len(issues))
	for _, issue := range issues {
		parts = append(parts, issue.Rule+": "+issue.Message)
	}
	return strings.Join(parts, "\n")
}

func mustMkdirAllValidate(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
}

func mustWriteFileValidate(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
