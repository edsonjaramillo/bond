package application

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

type result struct {
	exitCode int
	stdout   string
	stderr   string
}

func runApplication(t *testing.T, arguments ...string) result {
	t.Helper()

	return runApplicationWithEnvironment(t, []string{"HOME=/home/tester"}, arguments...)
}

func runApplicationWithEnvironment(t *testing.T, environment []string, arguments ...string) result {
	t.Helper()

	return runApplicationInDirectory(t, "/project", environment, "", arguments...)
}

func runApplicationInDirectory(t *testing.T, directory string, environment []string, stdin string, arguments ...string) result {
	t.Helper()

	return runApplicationWithDependencies(t, directory, environment, stdin, Dependencies{}, arguments...)
}

func runApplicationWithDependencies(t *testing.T, directory string, environment []string, stdin string, dependencies Dependencies, arguments ...string) result {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run(context.Background(), Invocation{
		Arguments:        arguments,
		Environment:      environment,
		WorkingDirectory: directory,
		Stdin:            strings.NewReader(stdin),
		Stdout:           &stdout,
		Stderr:           &stderr,
	}, dependencies)

	return result{
		exitCode: exitCode,
		stdout:   stdout.String(),
		stderr:   stderr.String(),
	}
}

func TestBareCommandsPrintHelp(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		arguments []string
		usage     string
	}{
		{name: "root", arguments: nil, usage: "Usage:\n  bond"},
		{name: "skills", arguments: []string{"skills"}, usage: "Usage:\n  bond skills"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := runApplication(t, test.arguments...)
			if got.exitCode != 0 {
				t.Fatalf("exit code = %d, want 0; stderr = %q", got.exitCode, got.stderr)
			}
			if !strings.Contains(got.stdout, test.usage) {
				t.Errorf("stdout = %q, want help containing %q", got.stdout, test.usage)
			}
			if got.stderr != "" {
				t.Errorf("stderr = %q, want empty", got.stderr)
			}
		})
	}
}

func TestVersionInterfacesPrintOnlyVersion(t *testing.T) {
	t.Parallel()

	for _, arguments := range [][]string{{"version"}, {"--version"}} {
		got := runApplication(t, arguments...)
		if got.exitCode != 0 {
			t.Fatalf("Run(%q) exit code = %d, want 0; stderr = %q", arguments, got.exitCode, got.stderr)
		}
		if got.stdout != "dev\n" {
			t.Errorf("Run(%q) stdout = %q, want %q", arguments, got.stdout, "dev\n")
		}
		if got.stderr != "" {
			t.Errorf("Run(%q) stderr = %q, want empty", arguments, got.stderr)
		}
	}
}

func TestVersionDependencyCanBeReplaced(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run(context.Background(), Invocation{
		Arguments: []string{"version"},
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
	}, Dependencies{Version: "v1.2.3"})

	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}
	if got := stdout.String(); got != "v1.2.3\n" {
		t.Errorf("stdout = %q, want %q", got, "v1.2.3\n")
	}
}

func TestCompletionScripts(t *testing.T) {
	t.Parallel()

	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		t.Run(shell, func(t *testing.T) {
			t.Parallel()

			got := runApplication(t, "completion", shell)
			if got.exitCode != 0 {
				t.Fatalf("exit code = %d, want 0; stderr = %q", got.exitCode, got.stderr)
			}
			if strings.TrimSpace(got.stdout) == "" {
				t.Error("stdout is empty, want a native completion script")
			}
			if got.stderr != "" {
				t.Errorf("stderr = %q, want empty", got.stderr)
			}
		})
	}
}

func writeSkill(t *testing.T, directory, name, description string) {
	t.Helper()

	contents := "---\nname: " + name + "\ndescription: " + description + "\n---\n"
	writeSkillDocument(t, directory, []byte(contents))
}

func writeSkillDocument(t *testing.T, directory string, contents []byte) {
	t.Helper()

	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatalf("create Skill directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(directory, "SKILL.md"), contents, 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
}

func TestMissingStoreListsAsEmptyWithoutCreatingIt(t *testing.T) {
	t.Parallel()

	configDirectory := t.TempDir()
	store := filepath.Join(configDirectory, "bond", "skills")

	got := runApplicationWithEnvironment(
		t,
		[]string{"XDG_CONFIG_HOME=" + configDirectory},
		"skills", "list", "--store",
	)

	if got.exitCode != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", got.exitCode, got.stderr)
	}
	if got.stdout != "No skills found.\n" {
		t.Errorf("stdout = %q, want %q", got.stdout, "No skills found.\n")
	}
	if got.stderr != "" {
		t.Errorf("stderr = %q, want empty", got.stderr)
	}
	if _, err := os.Stat(store); !os.IsNotExist(err) {
		t.Errorf("Store was created or could not be inspected: %v", err)
	}
}

func TestStoreUsesPlatformUserConfigurationDirectoryWithoutXDG(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	configDirectory := filepath.Join(home, ".config")
	if runtime.GOOS == "darwin" {
		configDirectory = filepath.Join(home, "Library", "Application Support")
	}
	writeSkill(t, filepath.Join(configDirectory, "bond", "skills", "review"), "review", "Review changes")

	got := runApplicationWithEnvironment(t, []string{"HOME=" + home}, "skills", "list", "--store")

	if got.exitCode != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", got.exitCode, got.stderr)
	}
	if got.stdout != "review\n" {
		t.Errorf("stdout = %q, want %q", got.stdout, "review\n")
	}
	if got.stderr != "" {
		t.Errorf("stderr = %q, want empty", got.stderr)
	}
}

func TestStoreListingPrintsTopLevelAndOrganizedSkillsInLexicalOrder(t *testing.T) {
	t.Parallel()

	configDirectory := t.TempDir()
	store := filepath.Join(configDirectory, "bond", "skills")
	writeSkill(t, filepath.Join(store, "zebra"), "zebra", "Top-level Skill")
	writeSkill(t, filepath.Join(store, "backend", "review"), "review", "Backend review")
	writeSkill(t, filepath.Join(store, "frontend", "review"), "review", "Frontend review")
	if err := os.Mkdir(filepath.Join(store, "empty-org"), 0o755); err != nil {
		t.Fatalf("create empty Organization: %v", err)
	}

	got := runApplicationWithEnvironment(
		t,
		[]string{"XDG_CONFIG_HOME=" + configDirectory},
		"skills", "list", "--store",
	)

	if got.exitCode != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", got.exitCode, got.stderr)
	}
	if got.stdout != "backend/review\nfrontend/review\nzebra\n" {
		t.Errorf("stdout = %q, want lexical Store-relative paths", got.stdout)
	}
	if got.stderr != "" {
		t.Errorf("stderr = %q, want empty", got.stderr)
	}
}

func TestStoreListingValidatesSkillDocuments(t *testing.T) {
	t.Parallel()

	configDirectory := t.TempDir()
	store := filepath.Join(configDirectory, "bond", "skills")
	writeSkillDocument(t, filepath.Join(store, "extended"), []byte("---\r\nname: ' extended '\r\ndescription: ' Useful Skill '\r\nmetadata:\r\n  tags: [one, 2]\r\n---\r\nBody\r\n"))
	writeSkillDocument(t, filepath.Join(store, "bad-yaml"), []byte("---\nname: [\ndescription: broken\n---\n"))
	writeSkillDocument(t, filepath.Join(store, "invalid-utf8"), append([]byte("---\nname: invalid-utf8\ndescription: invalid\n---\n"), 0xff))
	writeSkillDocument(t, filepath.Join(store, "no-frontmatter"), []byte("# no frontmatter\n"))
	writeSkillDocument(t, filepath.Join(store, "not-mapping"), []byte("---\n- name\n- description\n---\n"))
	writeSkillDocument(t, filepath.Join(store, "missing-name"), []byte("---\ndescription: present\n---\n"))
	writeSkillDocument(t, filepath.Join(store, "missing-description"), []byte("---\nname: missing-description\n---\n"))
	writeSkillDocument(t, filepath.Join(store, "non-string"), []byte("---\nname: non-string\ndescription: 42\n---\n"))
	writeSkillDocument(t, filepath.Join(store, "empty-description"), []byte("---\nname: empty-description\ndescription: '   '\n---\n"))
	writeSkillDocument(t, filepath.Join(store, "Bad-Name"), []byte("---\nname: Bad-Name\ndescription: invalid name\n---\n"))
	writeSkillDocument(t, filepath.Join(store, "directory-name"), []byte("---\nname: different-name\ndescription: mismatch\n---\n"))
	if err := os.MkdirAll(filepath.Join(store, "org", "missing-document"), 0o755); err != nil {
		t.Fatalf("create Stored Skill without SKILL.md: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(store, "document-directory", "SKILL.md"), 0o755); err != nil {
		t.Fatalf("create directory in place of SKILL.md: %v", err)
	}
	writeSkill(t, filepath.Join(store, "symlink-document-target"), "symlink-document-target", "Target")
	if err := os.MkdirAll(filepath.Join(store, "org", "symlink-document"), 0o755); err != nil {
		t.Fatalf("create symlinked document Skill: %v", err)
	}
	if err := os.Symlink(filepath.Join(store, "symlink-document-target", "SKILL.md"), filepath.Join(store, "org", "symlink-document", "SKILL.md")); err != nil {
		t.Fatalf("create symlinked SKILL.md: %v", err)
	}

	got := runApplicationWithEnvironment(t, []string{"XDG_CONFIG_HOME=" + configDirectory}, "skills", "list", "--store")

	if got.exitCode != 1 {
		t.Errorf("exit code = %d, want 1", got.exitCode)
	}
	if got.stdout != "extended\nsymlink-document-target\n" {
		t.Errorf("stdout = %q, want valid Stored Skills in lexical order", got.stdout)
	}
	for _, malformedPath := range []string{
		"Bad-Name:", "bad-yaml:", "directory-name:", "document-directory:", "empty-description:",
		"invalid-utf8:", "missing-description:", "missing-name:", "no-frontmatter:", "non-string:",
		"not-mapping:", "org/missing-document:", "org/symlink-document:",
	} {
		if !strings.Contains(got.stderr, malformedPath) {
			t.Errorf("stderr = %q, want diagnostic for %q", got.stderr, malformedPath)
		}
	}
}

func TestStoreListingValidatesSkillResources(t *testing.T) {
	t.Parallel()

	configDirectory := t.TempDir()
	store := filepath.Join(configDirectory, "bond", "skills")

	validDirectory := filepath.Join(store, "resource-valid")
	writeSkill(t, validDirectory, "resource-valid", "Valid resources")
	if err := os.Mkdir(filepath.Join(validDirectory, "docs"), 0o755); err != nil {
		t.Fatalf("create resource directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(validDirectory, "docs", "guide.txt"), []byte("guide"), 0o644); err != nil {
		t.Fatalf("write resource file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(validDirectory, "run.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write executable resource: %v", err)
	}
	if err := os.Symlink("docs/guide.txt", filepath.Join(validDirectory, "guide-link")); err != nil {
		t.Fatalf("create safe file symlink: %v", err)
	}
	if err := os.Symlink("docs", filepath.Join(validDirectory, "docs-link")); err != nil {
		t.Fatalf("create safe directory symlink: %v", err)
	}
	if err := os.Symlink("guide-link", filepath.Join(validDirectory, "guide-chain")); err != nil {
		t.Fatalf("create safe chained symlink: %v", err)
	}

	outside := filepath.Join(configDirectory, "outside.txt")
	if err := os.WriteFile(outside, []byte("outside"), 0o644); err != nil {
		t.Fatalf("write outside target: %v", err)
	}
	for _, name := range []string{"absolute-link", "escaping-link", "broken-link", "cyclic-link", "growing-cycle", "special-entry"} {
		writeSkill(t, filepath.Join(store, name), name, "Invalid resource")
	}
	if err := os.Symlink(filepath.Join(validDirectory, "docs", "guide.txt"), filepath.Join(store, "absolute-link", "resource")); err != nil {
		t.Fatalf("create absolute resource symlink: %v", err)
	}
	if err := os.Symlink(filepath.Join("..", "..", "..", "outside.txt"), filepath.Join(store, "escaping-link", "resource")); err != nil {
		t.Fatalf("create escaping resource symlink: %v", err)
	}
	if err := os.Symlink("absent", filepath.Join(store, "broken-link", "resource")); err != nil {
		t.Fatalf("create broken resource symlink: %v", err)
	}
	if err := os.Symlink("second", filepath.Join(store, "cyclic-link", "first")); err != nil {
		t.Fatalf("create first cyclic resource symlink: %v", err)
	}
	if err := os.Symlink("first", filepath.Join(store, "cyclic-link", "second")); err != nil {
		t.Fatalf("create second cyclic resource symlink: %v", err)
	}
	if err := os.Symlink(filepath.Join("resource", "child"), filepath.Join(store, "growing-cycle", "resource")); err != nil {
		t.Fatalf("create growing cyclic resource symlink: %v", err)
	}
	if err := syscall.Mkfifo(filepath.Join(store, "special-entry", "resource.fifo"), 0o600); err != nil {
		t.Fatalf("create special resource entry: %v", err)
	}

	got := runApplicationWithEnvironment(t, []string{"XDG_CONFIG_HOME=" + configDirectory}, "skills", "list", "--store")

	if got.exitCode != 1 {
		t.Errorf("exit code = %d, want 1", got.exitCode)
	}
	if got.stdout != "resource-valid\n" {
		t.Errorf("stdout = %q, want valid resource Skill", got.stdout)
	}
	for _, malformedPath := range []string{"absolute-link:", "broken-link:", "cyclic-link:", "escaping-link:", "growing-cycle:", "special-entry:"} {
		if !strings.Contains(got.stderr, malformedPath) {
			t.Errorf("stderr = %q, want diagnostic for %q", got.stderr, malformedPath)
		}
	}
}

func TestStoreListingReportsMalformedCandidatesAlongsideValidResults(t *testing.T) {
	t.Parallel()

	configDirectory := t.TempDir()
	store := filepath.Join(configDirectory, "bond", "skills")
	writeSkill(t, filepath.Join(store, "valid"), "valid", "Valid Skill")
	if err := os.WriteFile(filepath.Join(store, "unexpected.txt"), []byte("not a Skill"), 0o644); err != nil {
		t.Fatalf("write unexpected Store entry: %v", err)
	}
	if err := os.Symlink("valid", filepath.Join(store, "linked-skill")); err != nil {
		t.Fatalf("create symlinked Stored Skill: %v", err)
	}
	if err := os.Mkdir(filepath.Join(store, "team"), 0o755); err != nil {
		t.Fatalf("create Organization: %v", err)
	}
	if err := os.WriteFile(filepath.Join(store, "team", "notes.txt"), []byte("unexpected"), 0o644); err != nil {
		t.Fatalf("write unexpected Organization entry: %v", err)
	}
	if err := os.Symlink("../valid", filepath.Join(store, "team", "linked")); err != nil {
		t.Fatalf("create symlinked Organization entry: %v", err)
	}
	writeSkill(t, filepath.Join(store, "team", "nested", "too-deep"), "too-deep", "Excessively nested Skill")

	got := runApplicationWithEnvironment(
		t,
		[]string{"XDG_CONFIG_HOME=" + configDirectory},
		"skills", "list", "--store",
	)

	if got.exitCode != 1 {
		t.Errorf("exit code = %d, want 1", got.exitCode)
	}
	if got.stdout != "valid\n" {
		t.Errorf("stdout = %q, want the valid Stored Skill", got.stdout)
	}
	for _, malformedPath := range []string{"linked-skill:", "team/linked:", "team/nested:", "team/notes.txt:", "unexpected.txt:"} {
		if !strings.Contains(got.stderr, malformedPath) {
			t.Errorf("stderr = %q, want diagnostic for %q", got.stderr, malformedPath)
		}
	}
	if strings.Contains(got.stderr, "Usage:") || strings.Contains(got.stderr, "Error:") {
		t.Errorf("stderr = %q, want concise plain-text diagnostics", got.stderr)
	}
}

func TestMissingOrEmptyProjectListsAsEmptyWithoutCreatingInfrastructure(t *testing.T) {
	t.Parallel()

	for _, prepare := range []bool{false, true} {
		project := t.TempDir()
		if prepare {
			if err := os.MkdirAll(filepath.Join(project, ".agents", "skills"), 0o755); err != nil {
				t.Fatalf("create empty Project collection: %v", err)
			}
		}

		got := runApplicationInDirectory(t, project, []string{"HOME=" + t.TempDir()}, "", "skills", "list")

		if got.exitCode != 0 || got.stdout != "No skills found.\n" || got.stderr != "" {
			t.Errorf("prepared = %v: exit code = %d, stdout = %q, stderr = %q", prepare, got.exitCode, got.stdout, got.stderr)
		}
		if !prepare {
			if _, err := os.Lstat(filepath.Join(project, ".agents")); !os.IsNotExist(err) {
				t.Errorf("Project infrastructure was created: %v", err)
			}
		}
	}
}

func TestProjectListingUsesExactWorkingDirectoryAndPrintsManagedAndUnmanagedSkills(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	writeSkill(t, filepath.Join(parent, ".agents", "skills", "parent-skill"), "parent-skill", "Parent Skill")
	project := filepath.Join(parent, "child")
	writeSkill(t, filepath.Join(project, ".agents", "skills", "zebra"), "zebra", "Unmanaged Skill")
	writeSkill(t, filepath.Join(project, ".agents", "skills", "alpha"), "alpha", "Managed Skill")
	if err := os.WriteFile(filepath.Join(project, ".agents", "bond-manifest.json"), []byte(`{"version":1,"skills":[{"name":"alpha"}]}`), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	got := runApplicationInDirectory(t, project, []string{"HOME=" + t.TempDir()}, "", "skills", "list")

	if got.exitCode != 0 || got.stdout != "alpha\nzebra\n" || got.stderr != "" {
		t.Errorf("exit code = %d, stdout = %q, stderr = %q", got.exitCode, got.stdout, got.stderr)
	}
}

func TestProjectListingReportsMalformedCandidatesAlongsideValidSymlinkedSkills(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	collection := filepath.Join(project, ".agents", "skills")
	outside := t.TempDir()
	writeSkill(t, filepath.Join(outside, "linked"), "linked", "Linked Skill")
	if err := os.MkdirAll(collection, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "linked"), filepath.Join(collection, "linked")); err != nil {
		t.Fatalf("link Project Skill: %v", err)
	}
	writeSkill(t, filepath.Join(collection, "valid"), "valid", "Valid Skill")
	writeSkillDocument(t, filepath.Join(collection, "bad-frontmatter"), []byte("not frontmatter\n"))
	if err := os.WriteFile(filepath.Join(collection, "unexpected.txt"), []byte("unexpected"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("missing", filepath.Join(collection, "broken")); err != nil {
		t.Fatal(err)
	}

	got := runApplicationInDirectory(t, project, []string{"HOME=" + t.TempDir()}, "", "skills", "list")

	if got.exitCode != 1 || got.stdout != "linked\nvalid\n" {
		t.Errorf("exit code = %d, stdout = %q, want partial lexical results", got.exitCode, got.stdout)
	}
	wantDiagnostics := []string{"bad-frontmatter:", "broken:", "unexpected.txt:"}
	last := -1
	for _, diagnostic := range wantDiagnostics {
		index := strings.Index(got.stderr, diagnostic)
		if index <= last {
			t.Errorf("stderr = %q, want ordered diagnostic %q", got.stderr, diagnostic)
		}
		last = index
	}
}

func TestProjectListingRejectsSymlinkedInfrastructure(t *testing.T) {
	t.Parallel()

	for _, infrastructure := range []string{".agents", filepath.Join(".agents", "skills")} {
		t.Run(infrastructure, func(t *testing.T) {
			t.Parallel()

			project := t.TempDir()
			target := t.TempDir()
			if infrastructure == ".agents" {
				if err := os.Symlink(target, filepath.Join(project, ".agents")); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.Mkdir(filepath.Join(project, ".agents"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, filepath.Join(project, infrastructure)); err != nil {
					t.Fatal(err)
				}
			}

			got := runApplicationInDirectory(t, project, []string{"HOME=" + t.TempDir()}, "", "skills", "list")

			if got.exitCode != 1 || got.stdout != "" || !strings.Contains(got.stderr, "must not be a symlink") {
				t.Errorf("exit code = %d, stdout = %q, stderr = %q", got.exitCode, got.stdout, got.stderr)
			}
		})
	}
}

func TestEditOpensMalformedUnmanagedProjectSkillAndRetainsValidChanges(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	skillDirectory := filepath.Join(project, ".agents", "skills", "review")
	writeSkillDocument(t, skillDirectory, []byte("malformed\n"))
	editorDirectory := t.TempDir()
	editor := filepath.Join(editorDirectory, "repair editor")
	script := "#!/bin/sh\nprintf 'editor output\\n'\nprintf '%s\\n' '---' 'name: review' 'description: Repaired Skill' '---' > SKILL.md\n"
	if err := os.WriteFile(editor, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	got := runApplicationInDirectory(t, project, []string{"HOME=" + t.TempDir(), "EDITOR='" + editor + "'"}, "", "skills", "edit", "review")

	if got.exitCode != 0 || got.stdout != "editor output\n" || got.stderr != "" {
		t.Errorf("exit code = %d, stdout = %q, stderr = %q", got.exitCode, got.stdout, got.stderr)
	}
}

func TestEditFollowsAProjectSkillSymlinkAndAcceptsItsExactBasename(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	collection := filepath.Join(project, ".agents", "skills")
	target := filepath.Join(t.TempDir(), "Review")
	writeSkillDocument(t, target, []byte("malformed\n"))
	if err := os.MkdirAll(collection, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(collection, "Review")); err != nil {
		t.Fatal(err)
	}
	editor := filepath.Join(t.TempDir(), "editor")
	if err := os.WriteFile(editor, []byte("#!/bin/sh\nprintf 'changed through link\\n' > SKILL.md\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := runApplicationInDirectory(t, project, []string{"HOME=" + t.TempDir(), "EDITOR=" + editor}, "", "skills", "edit", "Review")

	if got.exitCode != 1 || !strings.Contains(got.stderr, "Review:") {
		t.Errorf("exit code = %d, stderr = %q; want retained post-edit validation failure", got.exitCode, got.stderr)
	}
	contents, err := os.ReadFile(filepath.Join(target, "SKILL.md"))
	if err != nil || string(contents) != "changed through link\n" {
		t.Errorf("linked SKILL.md contents = %q, err = %v", contents, err)
	}
}

func TestEditFailuresRetainUserChanges(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		editor string
		script string
	}{
		{name: "unset editor"},
		{name: "editor exit", script: "#!/bin/sh\nprintf 'changed by failed editor\\n' > SKILL.md\nexit 7\n"},
		{name: "invalid result", script: "#!/bin/sh\nprintf 'changed but invalid\\n' > SKILL.md\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			project := t.TempDir()
			skillDirectory := filepath.Join(project, ".agents", "skills", "review")
			writeSkill(t, skillDirectory, "review", "Original")
			environment := []string{"HOME=" + t.TempDir()}
			if test.script != "" {
				test.editor = filepath.Join(t.TempDir(), "editor")
				if err := os.WriteFile(test.editor, []byte(test.script), 0o755); err != nil {
					t.Fatal(err)
				}
				environment = append(environment, "EDITOR="+test.editor)
			}

			got := runApplicationInDirectory(t, project, environment, "", "skills", "edit", "review")

			if got.exitCode != 1 || got.stdout != "" || got.stderr == "" {
				t.Errorf("exit code = %d, stdout = %q, stderr = %q", got.exitCode, got.stdout, got.stderr)
			}
			contents, err := os.ReadFile(filepath.Join(skillDirectory, "SKILL.md"))
			if err != nil {
				t.Fatal(err)
			}
			if test.script == "" && !strings.Contains(string(contents), "Original") {
				t.Errorf("unset editor changed content: %q", contents)
			}
			if test.script != "" && !strings.Contains(string(contents), "changed") {
				t.Errorf("editor changes were not retained: %q", contents)
			}
		})
	}
}

func TestNewCreatesTopLevelAndSkillDraftsUnderAnOrganizationWithoutAnEditor(t *testing.T) {
	t.Parallel()

	for _, storedPath := range []string{"review", "frontend/review"} {
		t.Run(storedPath, func(t *testing.T) {
			t.Parallel()

			configDirectory := t.TempDir()
			got := runApplicationWithEnvironment(t, []string{"XDG_CONFIG_HOME=" + configDirectory}, "skills", "new", storedPath)

			if got.exitCode != 0 {
				t.Fatalf("exit code = %d, want 0; stderr = %q", got.exitCode, got.stderr)
			}
			if got.stdout != "" || got.stderr != "" {
				t.Errorf("stdout = %q, stderr = %q; want silent success", got.stdout, got.stderr)
			}
			document, err := os.ReadFile(filepath.Join(configDirectory, "bond", "skills", filepath.FromSlash(storedPath), "SKILL.md"))
			if err != nil {
				t.Fatalf("read Skill Draft: %v", err)
			}
			if string(document) != "---\nname: review\ndescription: \"\"\n---\n" {
				t.Errorf("SKILL.md = %q, want minimal Skill Draft", document)
			}
		})
	}
}

func TestNewRejectsInvalidStoredPathsBeforeCreatingTheStore(t *testing.T) {
	t.Parallel()

	for _, storedPath := range []string{"", "/review", "frontend/", "frontend//review", "./review", "../review", "frontend/../review", "one/two/review", "Bad-Name", "bad--name", "frontend/Bad-Name"} {
		t.Run(strings.ReplaceAll(storedPath, "/", "_"), func(t *testing.T) {
			t.Parallel()

			configDirectory := t.TempDir()
			got := runApplicationWithEnvironment(t, []string{"XDG_CONFIG_HOME=" + configDirectory}, "skills", "new", storedPath)

			if got.exitCode != 1 {
				t.Errorf("exit code = %d, want 1", got.exitCode)
			}
			if got.stdout != "" || got.stderr == "" {
				t.Errorf("stdout = %q, stderr = %q; want one diagnostic", got.stdout, got.stderr)
			}
			store := filepath.Join(configDirectory, "bond", "skills")
			if _, err := os.Stat(store); !os.IsNotExist(err) {
				t.Errorf("Store was created or could not be inspected: %v", err)
			}
		})
	}
}

func TestNewDoesNotModifyExistingEntries(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name       string
		storedPath string
		prepare    func(*testing.T, string)
		unchanged  func(*testing.T, string)
	}{
		{
			name:       "destination directory",
			storedPath: "review",
			prepare: func(t *testing.T, store string) {
				writeSkillDocument(t, filepath.Join(store, "review"), []byte("existing"))
			},
			unchanged: func(t *testing.T, store string) {
				contents, err := os.ReadFile(filepath.Join(store, "review", "SKILL.md"))
				if err != nil || string(contents) != "existing" {
					t.Errorf("existing destination changed: contents = %q, err = %v", contents, err)
				}
			},
		},
		{
			name:       "destination file",
			storedPath: "review",
			prepare: func(t *testing.T, store string) {
				if err := os.MkdirAll(store, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(store, "review"), []byte("existing"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			unchanged: func(t *testing.T, store string) {
				contents, err := os.ReadFile(filepath.Join(store, "review"))
				if err != nil || string(contents) != "existing" {
					t.Errorf("existing destination changed: contents = %q, err = %v", contents, err)
				}
			},
		},
		{
			name:       "organization file",
			storedPath: "frontend/review",
			prepare: func(t *testing.T, store string) {
				if err := os.MkdirAll(store, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(store, "frontend"), []byte("existing"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			unchanged: func(t *testing.T, store string) {
				contents, err := os.ReadFile(filepath.Join(store, "frontend"))
				if err != nil || string(contents) != "existing" {
					t.Errorf("existing Organization entry changed: contents = %q, err = %v", contents, err)
				}
			},
		},
		{
			name:       "Stored Skill used as Organization",
			storedPath: "frontend/review",
			prepare: func(t *testing.T, store string) {
				writeSkill(t, filepath.Join(store, "frontend"), "frontend", "Existing Skill")
			},
			unchanged: func(t *testing.T, store string) {
				if _, err := os.Stat(filepath.Join(store, "frontend", "review")); !os.IsNotExist(err) {
					t.Errorf("nested Skill Draft was created in an existing Stored Skill: %v", err)
				}
			},
		},
		{
			name:       "symlink used as Organization",
			storedPath: "frontend/review",
			prepare: func(t *testing.T, store string) {
				if err := os.MkdirAll(filepath.Join(store, "target"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("target", filepath.Join(store, "frontend")); err != nil {
					t.Fatal(err)
				}
			},
			unchanged: func(t *testing.T, store string) {
				if _, err := os.Stat(filepath.Join(store, "target", "review")); !os.IsNotExist(err) {
					t.Errorf("Skill Draft was created through an Organization symlink: %v", err)
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			configDirectory := t.TempDir()
			store := filepath.Join(configDirectory, "bond", "skills")
			test.prepare(t, store)

			got := runApplicationWithEnvironment(t, []string{"XDG_CONFIG_HOME=" + configDirectory}, "skills", "new", test.storedPath)

			if got.exitCode != 1 || got.stdout != "" || got.stderr == "" {
				t.Errorf("exit code = %d, stdout = %q, stderr = %q; want concise failure", got.exitCode, got.stdout, got.stderr)
			}
			test.unchanged(t, store)
		})
	}
}

func TestNewRunsQuotedEditorArgumentsDirectlyFromTheSkillDraftDirectory(t *testing.T) {
	t.Parallel()

	configDirectory := t.TempDir()
	editorDirectory := t.TempDir()
	editor := filepath.Join(editorDirectory, "record editor")
	record := filepath.Join(editorDirectory, "record")
	script := "#!/bin/sh\nprintf '%s\\n' \"$PWD\" \"$@\" > \"$RECORD\"\nprintf 'stdin=%s\\n' \"$(cat)\"\nprintf '\\nAuthored body\\n' >> SKILL.md\n"
	if err := os.WriteFile(editor, []byte(script), 0o755); err != nil {
		t.Fatalf("write editor: %v", err)
	}
	environment := []string{
		"XDG_CONFIG_HOME=" + configDirectory,
		"EDITOR='" + editor + "' --wait \"two words\" | touch should-not-exist",
		"RECORD=" + record,
		"PATH=" + os.Getenv("PATH"),
	}

	got := runApplicationInDirectory(t, t.TempDir(), environment, "hello", "skills", "new", "review")

	if got.exitCode != 1 {
		t.Fatalf("exit code = %d, want 1 because description remains empty; stderr = %q", got.exitCode, got.stderr)
	}
	if got.stdout != "stdin=hello\n" {
		t.Errorf("stdout = %q, want inherited editor output", got.stdout)
	}
	if !strings.Contains(got.stderr, "description") {
		t.Errorf("stderr = %q, want post-editor validation diagnostic", got.stderr)
	}
	skillDraftDirectory := filepath.Join(configDirectory, "bond", "skills", "review")
	recorded, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("read editor record: %v", err)
	}
	physicalSkillDraftDirectory, err := filepath.EvalSymlinks(skillDraftDirectory)
	if err != nil {
		t.Fatalf("resolve Skill Draft directory: %v", err)
	}
	wantRecord := physicalSkillDraftDirectory + "\n--wait\ntwo words\n|\ntouch\nshould-not-exist\nSKILL.md\n"
	if string(recorded) != wantRecord {
		t.Errorf("editor record = %q, want %q", recorded, wantRecord)
	}
	if _, err := os.Stat(filepath.Join(skillDraftDirectory, "should-not-exist")); !os.IsNotExist(err) {
		t.Errorf("shell operator was evaluated: %v", err)
	}
	document, err := os.ReadFile(filepath.Join(skillDraftDirectory, "SKILL.md"))
	if err != nil || !strings.Contains(string(document), "Authored body") {
		t.Errorf("edited Skill Draft was not retained: contents = %q, err = %v", document, err)
	}
}

func TestNewRetainsSkillDraftWhenEditorFails(t *testing.T) {
	t.Parallel()

	for _, editor := range []string{"/definitely/missing/editor", "/bin/sh -c 'exit 7'"} {
		t.Run(editor, func(t *testing.T) {
			t.Parallel()

			configDirectory := t.TempDir()
			got := runApplicationWithEnvironment(t, []string{"XDG_CONFIG_HOME=" + configDirectory, "EDITOR=" + editor}, "skills", "new", "review")

			if got.exitCode != 1 || got.stdout != "" || got.stderr == "" {
				t.Errorf("exit code = %d, stdout = %q, stderr = %q; want editor failure", got.exitCode, got.stdout, got.stderr)
			}
			if _, err := os.Stat(filepath.Join(configDirectory, "bond", "skills", "review", "SKILL.md")); err != nil {
				t.Errorf("Skill Draft was not retained: %v", err)
			}
		})
	}
}

func TestNewAcceptsValidContentAfterEditorExits(t *testing.T) {
	t.Parallel()

	configDirectory := t.TempDir()
	editorDirectory := t.TempDir()
	editor := filepath.Join(editorDirectory, "author")
	script := "#!/bin/sh\nprintf '%s\\n' '---' 'name: review' 'description: Authored Skill' '---' > SKILL.md\n"
	if err := os.WriteFile(editor, []byte(script), 0o755); err != nil {
		t.Fatalf("write editor: %v", err)
	}

	got := runApplicationWithEnvironment(t, []string{"XDG_CONFIG_HOME=" + configDirectory, "EDITOR=" + editor}, "skills", "new", "review")

	if got.exitCode != 0 || got.stdout != "" || got.stderr != "" {
		t.Errorf("exit code = %d, stdout = %q, stderr = %q; want silent success", got.exitCode, got.stdout, got.stderr)
	}
}

func TestAddInstallsOneStoredSkillAsAnAbsoluteManagedLink(t *testing.T) {
	t.Parallel()

	configDirectory := t.TempDir()
	store := filepath.Join(configDirectory, "bond", "skills")
	source := filepath.Join(store, "frontend", "review")
	writeSkill(t, source, "review", "Frontend review")
	project := t.TempDir()

	got := runApplicationInDirectory(t, project, []string{"XDG_CONFIG_HOME=" + configDirectory}, "", "skills", "add", "frontend/review")

	if got.exitCode != 0 || got.stdout != "" || got.stderr != "" {
		t.Fatalf("exit code = %d, stdout = %q, stderr = %q; want silent success", got.exitCode, got.stdout, got.stderr)
	}
	destination := filepath.Join(project, ".agents", "skills", "review")
	target, err := os.Readlink(destination)
	if err != nil {
		t.Fatalf("read installed Project Skill link: %v", err)
	}
	if target != source || !filepath.IsAbs(target) {
		t.Errorf("link target = %q, want absolute source %q", target, source)
	}
	manifest, err := os.ReadFile(filepath.Join(project, ".agents", "bond-manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	want := "{\n  \"version\": 1,\n  \"skills\": [\n    {\n      \"name\": \"review\",\n      \"source\": \"frontend/review\",\n      \"mode\": \"link\",\n      \"destination\": \".agents/skills/review\"\n    }\n  ]\n}\n"
	if string(manifest) != want {
		t.Errorf("manifest = %s, want %s", manifest, want)
	}
}

func TestAddCopiesStoredSkillAsIndependentManagedSnapshot(t *testing.T) {
	t.Parallel()

	configDirectory := t.TempDir()
	source := filepath.Join(configDirectory, "bond", "skills", "review")
	writeSkill(t, source, "review", "Review changes")
	resourceDirectory := filepath.Join(source, "resources")
	if err := os.Mkdir(resourceDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(resourceDirectory, "check.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho stored\n"), 0o751); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("resources", "check.sh"), filepath.Join(source, "check")); err != nil {
		t.Fatal(err)
	}
	project := t.TempDir()

	got := runApplicationInDirectory(t, project, []string{"XDG_CONFIG_HOME=" + configDirectory}, "", "skills", "add", "review", "--copy")

	if got.exitCode != 0 || got.stdout != "" || got.stderr != "" {
		t.Fatalf("exit code = %d, stdout = %q, stderr = %q; want silent success", got.exitCode, got.stdout, got.stderr)
	}
	destination := filepath.Join(project, ".agents", "skills", "review")
	if info, err := os.Lstat(destination); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("copied Project Skill info = %v, err = %v; want real directory", info, err)
	}
	contents, err := os.ReadFile(filepath.Join(destination, "resources", "check.sh"))
	if err != nil || string(contents) != "#!/bin/sh\necho stored\n" {
		t.Errorf("copied script = %q, err = %v", contents, err)
	}
	info, err := os.Stat(filepath.Join(destination, "resources", "check.sh"))
	if err != nil || info.Mode().Perm()&0o111 != 0o111 {
		t.Errorf("copied script mode = %v, err = %v; want executable bits preserved", info, err)
	}
	target, err := os.Readlink(filepath.Join(destination, "check"))
	if err != nil || target != filepath.Join("resources", "check.sh") {
		t.Errorf("copied resource symlink target = %q, err = %v", target, err)
	}

	if err := os.WriteFile(script, []byte("store changed\n"), 0o751); err != nil {
		t.Fatal(err)
	}
	if copied, err := os.ReadFile(filepath.Join(destination, "resources", "check.sh")); err != nil || string(copied) != "#!/bin/sh\necho stored\n" {
		t.Errorf("Store change affected copied Project Skill: contents = %q, err = %v", copied, err)
	}
	if err := os.WriteFile(filepath.Join(destination, "resources", "check.sh"), []byte("project changed\n"), 0o751); err != nil {
		t.Fatal(err)
	}
	if stored, err := os.ReadFile(script); err != nil || string(stored) != "store changed\n" {
		t.Errorf("Project change affected Stored Skill: contents = %q, err = %v", stored, err)
	}

	readOnlyDirectory := filepath.Join(source, "read-only")
	if err := os.Mkdir(readOnlyDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(readOnlyDirectory, "notes.txt"), []byte("preserved\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(readOnlyDirectory, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnlyDirectory, 0o755) })

	secondProject := t.TempDir()
	second := runApplicationInDirectory(t, secondProject, []string{"XDG_CONFIG_HOME=" + configDirectory}, "", "skills", "add", "review", "--copy")
	if second.exitCode != 0 {
		t.Fatalf("copy Skill with read-only directory: stderr = %q", second.stderr)
	}
	copiedReadOnly := filepath.Join(secondProject, ".agents", "skills", "review", "read-only")
	t.Cleanup(func() { _ = os.Chmod(copiedReadOnly, 0o755) })
	if contents, err := os.ReadFile(filepath.Join(copiedReadOnly, "notes.txt")); err != nil || string(contents) != "preserved\n" {
		t.Errorf("copied read-only resource = %q, err = %v", contents, err)
	}
	if info, err := os.Stat(copiedReadOnly); err != nil || info.Mode().Perm() != 0o555 {
		t.Errorf("copied directory mode = %v, err = %v; want 0555", info, err)
	}

	manifest, err := os.ReadFile(filepath.Join(project, ".agents", "bond-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(manifest), `"mode": "copy"`) {
		t.Errorf("manifest = %s, want copy mode", manifest)
	}
}

func TestAddValidatesOnlyTheSelectedStoredSkill(t *testing.T) {
	t.Parallel()

	configDirectory := t.TempDir()
	store := filepath.Join(configDirectory, "bond", "skills")
	writeSkill(t, filepath.Join(store, "review"), "review", "Review changes")
	writeSkillDocument(t, filepath.Join(store, "unrelated"), []byte("malformed\n"))
	project := t.TempDir()
	writeSkillDocument(t, filepath.Join(project, ".agents", "skills", "also-unrelated"), []byte("malformed\n"))

	got := runApplicationInDirectory(t, project, []string{"XDG_CONFIG_HOME=" + configDirectory}, "", "skills", "add", "review")

	if got.exitCode != 0 || got.stdout != "" || got.stderr != "" {
		t.Fatalf("exit code = %d, stdout = %q, stderr = %q; want unrelated malformed Skills ignored", got.exitCode, got.stdout, got.stderr)
	}
}

func TestAddRejectsDestinationOwnershipAndInvalidManifestsWithoutChangingProject(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name           string
		manifest       string
		createExisting bool
		wantDiagnostic string
	}{
		{name: "unmanaged destination", createExisting: true, wantDiagnostic: "already exists"},
		{name: "stale ownership", manifest: `{"version":1,"skills":[{"name":"review","source":"old/review","mode":"link","destination":".agents/skills/review"}]}`, wantDiagnostic: "ownership"},
		{name: "corrupt manifest", manifest: `{not json`, wantDiagnostic: "manifest"},
		{name: "unsupported manifest", manifest: `{"version":2,"skills":[]}`, wantDiagnostic: "version"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			configDirectory := t.TempDir()
			writeSkill(t, filepath.Join(configDirectory, "bond", "skills", "review"), "review", "Review changes")
			project := t.TempDir()
			if test.createExisting {
				writeSkill(t, filepath.Join(project, ".agents", "skills", "review"), "review", "User maintained")
			}
			if test.manifest != "" {
				if err := os.MkdirAll(filepath.Join(project, ".agents"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(project, ".agents", "bond-manifest.json"), []byte(test.manifest), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			got := runApplicationInDirectory(t, project, []string{"XDG_CONFIG_HOME=" + configDirectory}, "", "skills", "add", "review")

			if got.exitCode != 1 || got.stdout != "" || !strings.Contains(got.stderr, test.wantDiagnostic) {
				t.Errorf("exit code = %d, stdout = %q, stderr = %q", got.exitCode, got.stdout, got.stderr)
			}
			if test.manifest != "" {
				contents, err := os.ReadFile(filepath.Join(project, ".agents", "bond-manifest.json"))
				if err != nil || string(contents) != test.manifest {
					t.Errorf("manifest changed: contents = %q, err = %v", contents, err)
				}
			}
			if !test.createExisting {
				if _, err := os.Lstat(filepath.Join(project, ".agents", "skills", "review")); !os.IsNotExist(err) {
					t.Errorf("destination was created: %v", err)
				}
			}
		})
	}
}

func TestAddInstallsMultipleStoredSkillsWithOneManifestTransition(t *testing.T) {
	t.Parallel()

	configDirectory := t.TempDir()
	store := filepath.Join(configDirectory, "bond", "skills")
	firstSource := filepath.Join(store, "frontend", "review")
	secondSource := filepath.Join(store, "deploy")
	writeSkill(t, firstSource, "review", "Review changes")
	writeSkill(t, secondSource, "deploy", "Deploy changes")
	project := t.TempDir()

	got := runApplicationInDirectory(t, project, []string{"XDG_CONFIG_HOME=" + configDirectory}, "", "skills", "add", "frontend/review", "deploy")

	if got.exitCode != 0 || got.stdout != "" || got.stderr != "" {
		t.Fatalf("exit code = %d, stdout = %q, stderr = %q; want silent success", got.exitCode, got.stdout, got.stderr)
	}
	for destination, source := range map[string]string{
		filepath.Join(project, ".agents", "skills", "review"): firstSource,
		filepath.Join(project, ".agents", "skills", "deploy"): secondSource,
	} {
		target, err := os.Readlink(destination)
		if err != nil || target != source {
			t.Errorf("link %q target = %q, err = %v; want %q", destination, target, err, source)
		}
	}
	manifest, err := os.ReadFile(filepath.Join(project, ".agents", "bond-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(manifest), `"mode": "link"`) != 2 {
		t.Errorf("manifest = %s, want two linked Managed Skills", manifest)
	}
}

func TestAddCopyAppliesToEverySkillInBatch(t *testing.T) {
	t.Parallel()

	configDirectory := t.TempDir()
	store := filepath.Join(configDirectory, "bond", "skills")
	writeSkill(t, filepath.Join(store, "review"), "review", "Review changes")
	writeSkill(t, filepath.Join(store, "deploy"), "deploy", "Deploy changes")
	project := t.TempDir()

	got := runApplicationInDirectory(t, project, []string{"XDG_CONFIG_HOME=" + configDirectory}, "", "skills", "add", "review", "deploy", "--copy")

	if got.exitCode != 0 || got.stdout != "" || got.stderr != "" {
		t.Fatalf("exit code = %d, stdout = %q, stderr = %q; want silent success", got.exitCode, got.stdout, got.stderr)
	}
	for _, name := range []string{"review", "deploy"} {
		info, err := os.Lstat(filepath.Join(project, ".agents", "skills", name))
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			t.Errorf("Project Skill %q info = %v, err = %v; want copied directory", name, info, err)
		}
	}
	manifest, err := os.ReadFile(filepath.Join(project, ".agents", "bond-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(manifest), `"mode": "copy"`) != 2 || strings.Contains(string(manifest), `"mode": "link"`) {
		t.Errorf("manifest = %s, want every installation in copy mode", manifest)
	}
}

func TestAddCopyRejectsUnsafeSourceWithoutChangingBatch(t *testing.T) {
	t.Parallel()

	configDirectory := t.TempDir()
	store := filepath.Join(configDirectory, "bond", "skills")
	writeSkill(t, filepath.Join(store, "review"), "review", "Review changes")
	unsafeSource := filepath.Join(store, "deploy")
	writeSkill(t, unsafeSource, "deploy", "Deploy changes")
	if err := os.Symlink(filepath.Join("..", "review", "SKILL.md"), filepath.Join(unsafeSource, "outside")); err != nil {
		t.Fatal(err)
	}
	project := t.TempDir()

	got := runApplicationInDirectory(t, project, []string{"XDG_CONFIG_HOME=" + configDirectory}, "", "skills", "add", "review", "deploy", "--copy")

	if got.exitCode != 1 || got.stdout != "" || !strings.Contains(got.stderr, "escapes") {
		t.Fatalf("exit code = %d, stdout = %q, stderr = %q; want unsafe source failure", got.exitCode, got.stdout, got.stderr)
	}
	if _, err := os.Lstat(filepath.Join(project, ".agents")); !os.IsNotExist(err) {
		t.Errorf("copy batch changed Project state: %v", err)
	}
}

func TestAddReportsAllBatchErrorsInArgumentOrderWithoutChangingProject(t *testing.T) {
	t.Parallel()

	configDirectory := t.TempDir()
	store := filepath.Join(configDirectory, "bond", "skills")
	writeSkillDocument(t, filepath.Join(store, "broken"), []byte("malformed\n"))
	writeSkill(t, filepath.Join(store, "frontend", "review"), "review", "Frontend review")
	writeSkill(t, filepath.Join(store, "backend", "review"), "review", "Backend review")
	writeSkill(t, filepath.Join(store, "deploy"), "deploy", "Deploy changes")
	project := t.TempDir()
	writeSkill(t, filepath.Join(project, ".agents", "skills", "deploy"), "deploy", "User maintained")

	got := runApplicationInDirectory(t, project, []string{"XDG_CONFIG_HOME=" + configDirectory}, "", "skills", "add", "broken", "frontend/review", "backend/review", "frontend/review", "deploy")

	if got.exitCode != 1 || got.stdout != "" {
		t.Fatalf("exit code = %d, stdout = %q, stderr = %q", got.exitCode, got.stdout, got.stderr)
	}
	ordered := []string{"broken:", "backend/review:", "frontend/review:", "deploy:"}
	position := -1
	for _, diagnostic := range ordered {
		next := strings.Index(got.stderr[position+1:], diagnostic)
		if next < 0 {
			t.Fatalf("stderr = %q, want diagnostic containing %q", got.stderr, diagnostic)
		}
		position += next + 1
	}
	if _, err := os.Lstat(filepath.Join(project, ".agents", "skills", "review")); !os.IsNotExist(err) {
		t.Errorf("batch created review destination: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(project, ".agents", "bond-manifest.json")); !os.IsNotExist(err) {
		t.Errorf("batch created manifest: %v", err)
	}
}

func TestAddHandledPublicationFailureRestoresPriorProjectState(t *testing.T) {
	t.Parallel()

	configDirectory := t.TempDir()
	store := filepath.Join(configDirectory, "bond", "skills")
	writeSkill(t, filepath.Join(store, "review"), "review", "Review changes")
	writeSkill(t, filepath.Join(store, "deploy"), "deploy", "Deploy changes")
	project := t.TempDir()

	got := runApplicationWithDependencies(t, project, []string{"XDG_CONFIG_HOME=" + configDirectory}, "", Dependencies{TransactionFailurePoint: afterFirstPublish}, "skills", "add", "review", "deploy")

	if got.exitCode != 1 || got.stdout != "" || got.stderr == "" {
		t.Fatalf("exit code = %d, stdout = %q, stderr = %q; want handled failure", got.exitCode, got.stdout, got.stderr)
	}
	if _, err := os.Lstat(filepath.Join(project, ".agents")); !os.IsNotExist(err) {
		t.Errorf("Project state was not restored: %v", err)
	}
}

func TestAddCopyHandledPublicationFailureRestoresPriorProjectState(t *testing.T) {
	t.Parallel()

	configDirectory := t.TempDir()
	store := filepath.Join(configDirectory, "bond", "skills")
	writeSkill(t, filepath.Join(store, "review"), "review", "Review changes")
	writeSkill(t, filepath.Join(store, "deploy"), "deploy", "Deploy changes")
	project := t.TempDir()

	got := runApplicationWithDependencies(t, project, []string{"XDG_CONFIG_HOME=" + configDirectory}, "", Dependencies{TransactionFailurePoint: afterFirstPublish}, "skills", "add", "review", "deploy", "--copy")

	if got.exitCode != 1 || got.stdout != "" || got.stderr == "" {
		t.Fatalf("exit code = %d, stdout = %q, stderr = %q; want handled failure", got.exitCode, got.stdout, got.stderr)
	}
	if _, err := os.Lstat(filepath.Join(project, ".agents")); !os.IsNotExist(err) {
		t.Errorf("Project state was not restored: %v", err)
	}
}

func TestAddRecoversInterruptedPublicationBeforeNextMutation(t *testing.T) {
	t.Parallel()

	configDirectory := t.TempDir()
	store := filepath.Join(configDirectory, "bond", "skills")
	writeSkill(t, filepath.Join(store, "review"), "review", "Review changes")
	writeSkill(t, filepath.Join(store, "deploy"), "deploy", "Deploy changes")
	project := t.TempDir()
	environment := []string{"XDG_CONFIG_HOME=" + configDirectory}

	interrupted := runApplicationWithDependencies(t, project, environment, "", Dependencies{TransactionInterruptionPoint: afterFirstPublish}, "skills", "add", "review", "deploy")
	if interrupted.exitCode != 1 || interrupted.stdout != "" || interrupted.stderr == "" {
		t.Fatalf("interrupted exit code = %d, stdout = %q, stderr = %q", interrupted.exitCode, interrupted.stdout, interrupted.stderr)
	}
	if _, err := os.Stat(filepath.Join(project, ".agents", "bond-journal.json")); err != nil {
		t.Fatalf("transaction journal was not retained: %v", err)
	}

	recovered := runApplicationInDirectory(t, project, environment, "", "skills", "add", "review", "deploy")
	if recovered.exitCode != 0 || recovered.stdout != "" || recovered.stderr != "" {
		t.Fatalf("recovered exit code = %d, stdout = %q, stderr = %q", recovered.exitCode, recovered.stdout, recovered.stderr)
	}
	for _, name := range []string{"review", "deploy"} {
		if _, err := os.Readlink(filepath.Join(project, ".agents", "skills", name)); err != nil {
			t.Errorf("Project Skill %q was not installed after recovery: %v", name, err)
		}
	}
	if _, err := os.Lstat(filepath.Join(project, ".agents", "bond-journal.json")); !os.IsNotExist(err) {
		t.Errorf("transaction journal remains after recovery: %v", err)
	}
}

func TestAddRecoversInterruptionsAtEveryJournaledPhaseBeforeNextMutation(t *testing.T) {
	for _, test := range []struct {
		name                 string
		interruptionPoint    string
		interruptedCommitted bool
	}{
		{name: "journal written", interruptionPoint: afterJournalWrite},
		{name: "first publication", interruptionPoint: afterFirstPublish},
		{name: "all publications", interruptionPoint: afterAllPublishes},
		{name: "manifest written", interruptionPoint: afterManifestWrite, interruptedCommitted: true},
		{name: "staging removed", interruptionPoint: afterStageRemoval, interruptedCommitted: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			configDirectory := t.TempDir()
			store := filepath.Join(configDirectory, "bond", "skills")
			for _, name := range []string{"base", "review", "deploy", "lint"} {
				writeSkill(t, filepath.Join(store, name), name, name+" changes")
			}
			project := t.TempDir()
			environment := []string{"XDG_CONFIG_HOME=" + configDirectory}
			initial := runApplicationInDirectory(t, project, environment, "", "skills", "add", "base")
			if initial.exitCode != 0 {
				t.Fatalf("initial exit code = %d, stderr = %q", initial.exitCode, initial.stderr)
			}

			interrupted := runApplicationWithDependencies(t, project, environment, "", Dependencies{TransactionInterruptionPoint: test.interruptionPoint}, "skills", "add", "review", "deploy")
			if interrupted.exitCode != 1 || interrupted.stdout != "" || interrupted.stderr == "" {
				t.Fatalf("interrupted exit code = %d, stdout = %q, stderr = %q", interrupted.exitCode, interrupted.stdout, interrupted.stderr)
			}

			recovered := runApplicationInDirectory(t, project, environment, "", "skills", "add", "lint")
			if recovered.exitCode != 0 || recovered.stdout != "" || recovered.stderr != "" {
				t.Fatalf("recovered exit code = %d, stdout = %q, stderr = %q", recovered.exitCode, recovered.stdout, recovered.stderr)
			}
			for _, name := range []string{"review", "deploy"} {
				_, err := os.Lstat(filepath.Join(project, ".agents", "skills", name))
				if test.interruptedCommitted && err != nil {
					t.Errorf("committed Project Skill %q is absent after recovery: %v", name, err)
				}
				if !test.interruptedCommitted && !os.IsNotExist(err) {
					t.Errorf("uncommitted Project Skill %q remains after recovery: %v", name, err)
				}
			}
			for _, name := range []string{"base", "lint"} {
				if _, err := os.Lstat(filepath.Join(project, ".agents", "skills", name)); err != nil {
					t.Errorf("Project Skill %q is absent after recovery: %v", name, err)
				}
			}
			if _, err := os.Lstat(filepath.Join(project, ".agents", "bond-journal.json")); !os.IsNotExist(err) {
				t.Errorf("transaction journal remains after recovery: %v", err)
			}
		})
	}
}

func TestAddCopyRecoversInterruptionsAtEveryJournaledPhaseBeforeNextMutation(t *testing.T) {
	for _, test := range []struct {
		name                 string
		interruptionPoint    string
		interruptedCommitted bool
	}{
		{name: "journal written", interruptionPoint: afterJournalWrite},
		{name: "first publication", interruptionPoint: afterFirstPublish},
		{name: "all publications", interruptionPoint: afterAllPublishes},
		{name: "manifest written", interruptionPoint: afterManifestWrite, interruptedCommitted: true},
		{name: "staging removed", interruptionPoint: afterStageRemoval, interruptedCommitted: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			configDirectory := t.TempDir()
			store := filepath.Join(configDirectory, "bond", "skills")
			for _, name := range []string{"base", "review", "deploy", "lint"} {
				writeSkill(t, filepath.Join(store, name), name, name+" changes")
			}
			project := t.TempDir()
			environment := []string{"XDG_CONFIG_HOME=" + configDirectory}
			initial := runApplicationInDirectory(t, project, environment, "", "skills", "add", "base", "--copy")
			if initial.exitCode != 0 {
				t.Fatalf("initial exit code = %d, stderr = %q", initial.exitCode, initial.stderr)
			}

			interrupted := runApplicationWithDependencies(t, project, environment, "", Dependencies{TransactionInterruptionPoint: test.interruptionPoint}, "skills", "add", "review", "deploy", "--copy")
			if interrupted.exitCode != 1 || interrupted.stdout != "" || interrupted.stderr == "" {
				t.Fatalf("interrupted exit code = %d, stdout = %q, stderr = %q", interrupted.exitCode, interrupted.stdout, interrupted.stderr)
			}

			recovered := runApplicationInDirectory(t, project, environment, "", "skills", "add", "lint", "--copy")
			if recovered.exitCode != 0 || recovered.stdout != "" || recovered.stderr != "" {
				t.Fatalf("recovered exit code = %d, stdout = %q, stderr = %q", recovered.exitCode, recovered.stdout, recovered.stderr)
			}
			for _, name := range []string{"review", "deploy"} {
				info, err := os.Lstat(filepath.Join(project, ".agents", "skills", name))
				if test.interruptedCommitted && (err != nil || !info.IsDir()) {
					t.Errorf("committed copied Project Skill %q is absent after recovery: info = %v, err = %v", name, info, err)
				}
				if !test.interruptedCommitted && !os.IsNotExist(err) {
					t.Errorf("uncommitted copied Project Skill %q remains after recovery: %v", name, err)
				}
			}
			for _, name := range []string{"base", "lint"} {
				if info, err := os.Lstat(filepath.Join(project, ".agents", "skills", name)); err != nil || !info.IsDir() {
					t.Errorf("copied Project Skill %q is absent: info = %v, err = %v", name, info, err)
				}
			}
			if _, err := os.Lstat(filepath.Join(project, ".agents", "bond-journal.json")); !os.IsNotExist(err) {
				t.Errorf("transaction journal remains after recovery: %v", err)
			}
		})
	}
}

func TestAddCopyRecoveryPreservesIdenticalPathRecreatedAfterInterruption(t *testing.T) {
	t.Parallel()

	configDirectory := t.TempDir()
	store := filepath.Join(configDirectory, "bond", "skills")
	writeSkill(t, filepath.Join(store, "review"), "review", "Review changes")
	writeSkill(t, filepath.Join(store, "lint"), "lint", "Lint changes")
	project := t.TempDir()
	environment := []string{"XDG_CONFIG_HOME=" + configDirectory}

	interrupted := runApplicationWithDependencies(t, project, environment, "", Dependencies{TransactionInterruptionPoint: afterFirstPublish}, "skills", "add", "review", "--copy")
	if interrupted.exitCode != 1 {
		t.Fatalf("interrupted exit code = %d, stderr = %q", interrupted.exitCode, interrupted.stderr)
	}
	destination := filepath.Join(project, ".agents", "skills", "review")
	if err := os.RemoveAll(destination); err != nil {
		t.Fatal(err)
	}
	writeSkill(t, destination, "review", "Review changes")

	recovery := runApplicationInDirectory(t, project, environment, "", "skills", "add", "lint", "--copy")
	if recovery.exitCode != 1 || !strings.Contains(recovery.stderr, "manual recovery") || !strings.Contains(recovery.stderr, "conflicts") {
		t.Fatalf("recovery exit code = %d, stderr = %q; want manual recovery", recovery.exitCode, recovery.stderr)
	}
	if contents, err := os.ReadFile(filepath.Join(destination, "SKILL.md")); err != nil || !strings.Contains(string(contents), "Review changes") {
		t.Errorf("recreated identical Project Skill changed: contents = %q, err = %v", contents, err)
	}
	if _, err := os.Lstat(filepath.Join(project, ".agents", "skills", "lint")); !os.IsNotExist(err) {
		t.Errorf("next mutation proceeded despite unresolved recovery: %v", err)
	}
}

func TestRecoveryPreservesStagedAndNewConflictingProjectSkills(t *testing.T) {
	t.Parallel()

	configDirectory := t.TempDir()
	store := filepath.Join(configDirectory, "bond", "skills")
	writeSkill(t, filepath.Join(store, "review"), "review", "Stored review")
	writeSkill(t, filepath.Join(store, "lint"), "lint", "Stored lint")
	project := t.TempDir()
	environment := []string{"XDG_CONFIG_HOME=" + configDirectory}

	interrupted := runApplicationWithDependencies(t, project, environment, "", Dependencies{TransactionInterruptionPoint: afterJournalWrite}, "skills", "add", "review")
	if interrupted.exitCode != 1 {
		t.Fatalf("interrupted exit code = %d, stderr = %q", interrupted.exitCode, interrupted.stderr)
	}
	staging, err := filepath.Glob(filepath.Join(project, ".agents", ".bond-stage-*"))
	if err != nil || len(staging) != 1 {
		t.Fatalf("staging paths = %q, err = %v; want one", staging, err)
	}
	stagedSkill := filepath.Join(staging[0], "review")
	if _, err := os.Lstat(stagedSkill); err != nil {
		t.Fatalf("staged Project Skill is absent: %v", err)
	}
	conflictingSkill := filepath.Join(project, ".agents", "skills", "review")
	writeSkill(t, conflictingSkill, "review", "Created after interruption")

	recovery := runApplicationInDirectory(t, project, environment, "", "skills", "add", "lint")
	if recovery.exitCode != 1 || recovery.stdout != "" || !strings.Contains(recovery.stderr, "manual recovery") || !strings.Contains(recovery.stderr, "review") {
		t.Fatalf("recovery exit code = %d, stdout = %q, stderr = %q", recovery.exitCode, recovery.stdout, recovery.stderr)
	}
	for _, path := range []string{stagedSkill, conflictingSkill, filepath.Join(project, ".agents", "bond-journal.json")} {
		if _, err := os.Lstat(path); err != nil {
			t.Errorf("recovery did not preserve %q: %v", path, err)
		}
	}
	if _, err := os.Lstat(filepath.Join(project, ".agents", "skills", "lint")); !os.IsNotExist(err) {
		t.Errorf("next mutation proceeded despite unresolved recovery: %v", err)
	}
}

func TestRecoveryRejectsSymlinkedTransactionStagingWithoutTouchingItsTarget(t *testing.T) {
	t.Parallel()

	configDirectory := t.TempDir()
	store := filepath.Join(configDirectory, "bond", "skills")
	writeSkill(t, filepath.Join(store, "review"), "review", "Stored review")
	writeSkill(t, filepath.Join(store, "lint"), "lint", "Stored lint")
	project := t.TempDir()
	environment := []string{"XDG_CONFIG_HOME=" + configDirectory}

	interrupted := runApplicationWithDependencies(t, project, environment, "", Dependencies{TransactionInterruptionPoint: afterJournalWrite}, "skills", "add", "review")
	if interrupted.exitCode != 1 {
		t.Fatalf("interrupted exit code = %d, stderr = %q", interrupted.exitCode, interrupted.stderr)
	}
	staging, err := filepath.Glob(filepath.Join(project, ".agents", ".bond-stage-*"))
	if err != nil || len(staging) != 1 {
		t.Fatalf("staging paths = %q, err = %v; want one", staging, err)
	}
	if err := os.RemoveAll(staging[0]); err != nil {
		t.Fatal(err)
	}
	target := t.TempDir()
	marker := filepath.Join(target, "review")
	if err := os.WriteFile(marker, []byte("outside Project"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, staging[0]); err != nil {
		t.Fatal(err)
	}

	recovery := runApplicationInDirectory(t, project, environment, "", "skills", "add", "lint")
	if recovery.exitCode != 1 || recovery.stdout != "" || !strings.Contains(recovery.stderr, "staging") || !strings.Contains(recovery.stderr, "symlink") {
		t.Fatalf("recovery exit code = %d, stdout = %q, stderr = %q", recovery.exitCode, recovery.stdout, recovery.stderr)
	}
	contents, err := os.ReadFile(marker)
	if err != nil || string(contents) != "outside Project" {
		t.Errorf("staging target changed: contents = %q, err = %v", contents, err)
	}
	if _, err := os.Lstat(filepath.Join(project, ".agents", "bond-journal.json")); err != nil {
		t.Errorf("journal was not retained after unsafe recovery: %v", err)
	}
}

func TestRecoveryPreservesContentRecreatedAtRemovedStagingPath(t *testing.T) {
	t.Parallel()

	configDirectory := t.TempDir()
	writeSkill(t, filepath.Join(configDirectory, "bond", "skills", "review"), "review", "Stored review")
	writeSkill(t, filepath.Join(configDirectory, "bond", "skills", "lint"), "lint", "Stored lint")
	project := t.TempDir()
	environment := []string{"XDG_CONFIG_HOME=" + configDirectory}

	interrupted := runApplicationWithDependencies(t, project, environment, "", Dependencies{TransactionInterruptionPoint: afterStageRemoval}, "skills", "add", "review")
	if interrupted.exitCode != 1 {
		t.Fatalf("interrupted exit code = %d, stderr = %q", interrupted.exitCode, interrupted.stderr)
	}
	journal, err := os.ReadFile(filepath.Join(project, ".agents", "bond-journal.json"))
	if err != nil {
		t.Fatal(err)
	}
	var state struct {
		StageDirectory string `json:"stageDirectory"`
	}
	if err := json.Unmarshal(journal, &state); err != nil {
		t.Fatal(err)
	}
	recreatedStage := filepath.Join(project, ".agents", state.StageDirectory)
	if err := os.Mkdir(recreatedStage, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(recreatedStage, "created-after-interruption")
	if err := os.WriteFile(marker, []byte("preserve me"), 0o644); err != nil {
		t.Fatal(err)
	}

	recovery := runApplicationInDirectory(t, project, environment, "", "skills", "add", "lint")
	if recovery.exitCode != 1 || !strings.Contains(recovery.stderr, "manual recovery") {
		t.Fatalf("recovery exit code = %d, stderr = %q", recovery.exitCode, recovery.stderr)
	}
	if contents, err := os.ReadFile(marker); err != nil || string(contents) != "preserve me" {
		t.Errorf("recreated staging content changed: contents = %q, err = %v", contents, err)
	}
	if _, err := os.Lstat(filepath.Join(project, ".agents", "bond-journal.json")); err != nil {
		t.Errorf("journal was not retained: %v", err)
	}
}

func TestRecoveryRetainsJournalWhenNewInfrastructureContentPreventsRollback(t *testing.T) {
	t.Parallel()

	configDirectory := t.TempDir()
	writeSkill(t, filepath.Join(configDirectory, "bond", "skills", "review"), "review", "Stored review")
	writeSkill(t, filepath.Join(configDirectory, "bond", "skills", "lint"), "lint", "Stored lint")
	project := t.TempDir()
	environment := []string{"XDG_CONFIG_HOME=" + configDirectory}

	interrupted := runApplicationWithDependencies(t, project, environment, "", Dependencies{TransactionInterruptionPoint: afterJournalWrite}, "skills", "add", "review")
	if interrupted.exitCode != 1 {
		t.Fatalf("interrupted exit code = %d, stderr = %q", interrupted.exitCode, interrupted.stderr)
	}
	marker := filepath.Join(project, ".agents", "skills", "created-after-interruption")
	if err := os.WriteFile(marker, []byte("preserve me"), 0o644); err != nil {
		t.Fatal(err)
	}

	recovery := runApplicationInDirectory(t, project, environment, "", "skills", "add", "lint")
	if recovery.exitCode != 1 || !strings.Contains(recovery.stderr, "manual recovery") || !strings.Contains(recovery.stderr, "move the conflicting path aside") {
		t.Fatalf("recovery exit code = %d, stderr = %q", recovery.exitCode, recovery.stderr)
	}
	if contents, err := os.ReadFile(marker); err != nil || string(contents) != "preserve me" {
		t.Errorf("new infrastructure content changed: contents = %q, err = %v", contents, err)
	}
	if _, err := os.Lstat(filepath.Join(project, ".agents", "bond-journal.json")); err != nil {
		t.Errorf("journal was not retained: %v", err)
	}
}

func TestListAndEditFailWhileProjectTransactionIsUnresolved(t *testing.T) {
	t.Parallel()

	configDirectory := t.TempDir()
	writeSkill(t, filepath.Join(configDirectory, "bond", "skills", "review"), "review", "Stored review")
	project := t.TempDir()
	environment := []string{"XDG_CONFIG_HOME=" + configDirectory, "EDITOR=" + filepath.Join(t.TempDir(), "must-not-run")}

	interrupted := runApplicationWithDependencies(t, project, environment, "", Dependencies{TransactionInterruptionPoint: afterJournalWrite}, "skills", "add", "review")
	if interrupted.exitCode != 1 {
		t.Fatalf("interrupted exit code = %d, stderr = %q", interrupted.exitCode, interrupted.stderr)
	}

	for _, arguments := range [][]string{{"skills", "list"}, {"skills", "edit", "review"}} {
		got := runApplicationInDirectory(t, project, environment, "", arguments...)
		if got.exitCode != 1 || got.stdout != "" || !strings.Contains(got.stderr, "requires recovery") {
			t.Errorf("arguments = %q, exit code = %d, stdout = %q, stderr = %q", arguments, got.exitCode, got.stdout, got.stderr)
		}
	}
	if _, err := os.Lstat(filepath.Join(project, ".agents", "bond-journal.json")); err != nil {
		t.Errorf("read-only commands changed unresolved transaction: %v", err)
	}
}

func TestAddReportsDestinationErrorsAlongsideCorruptManifest(t *testing.T) {
	t.Parallel()

	configDirectory := t.TempDir()
	writeSkill(t, filepath.Join(configDirectory, "bond", "skills", "review"), "review", "Review changes")
	project := t.TempDir()
	writeSkill(t, filepath.Join(project, ".agents", "skills", "review"), "review", "User maintained")
	manifestPath := filepath.Join(project, ".agents", "bond-manifest.json")
	if err := os.WriteFile(manifestPath, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := runApplicationInDirectory(t, project, []string{"XDG_CONFIG_HOME=" + configDirectory}, "", "skills", "add", "review")

	if got.exitCode != 1 || !strings.Contains(got.stderr, "review: destination") || !strings.Contains(got.stderr, "manifest") {
		t.Errorf("exit code = %d, stderr = %q; want destination and manifest diagnostics", got.exitCode, got.stderr)
	}
}

func TestAddTimesOutOnConcurrentProjectMutation(t *testing.T) {
	project := t.TempDir()
	lockedDirectory, err := os.Open(project)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := lockedDirectory.Close(); err != nil {
			t.Errorf("close locked Project: %v", err)
		}
	})
	if err := syscall.Flock(int(lockedDirectory.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := syscall.Flock(int(lockedDirectory.Fd()), syscall.LOCK_UN); err != nil {
			t.Errorf("unlock Project: %v", err)
		}
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Run(context.Background(), Invocation{
		Arguments:        []string{"skills", "add", "review"},
		Environment:      []string{"XDG_CONFIG_HOME=" + t.TempDir()},
		WorkingDirectory: project,
		Stdout:           &stdout,
		Stderr:           &stderr,
	}, Dependencies{ProjectLockTimeout: 25 * time.Millisecond})

	if exitCode != 1 || stdout.String() != "" || !strings.Contains(stderr.String(), project) || !strings.Contains(stderr.String(), "locked") {
		t.Errorf("exit code = %d, stdout = %q, stderr = %q; want lock timeout naming Project", exitCode, stdout.String(), stderr.String())
	}
}

func TestAddRejectsSymlinkedProjectInfrastructure(t *testing.T) {
	t.Parallel()

	configDirectory := t.TempDir()
	writeSkill(t, filepath.Join(configDirectory, "bond", "skills", "review"), "review", "Review changes")
	project := t.TempDir()
	if err := os.Symlink(t.TempDir(), filepath.Join(project, ".agents")); err != nil {
		t.Fatal(err)
	}

	got := runApplicationInDirectory(t, project, []string{"XDG_CONFIG_HOME=" + configDirectory}, "", "skills", "add", "review")

	if got.exitCode != 1 || got.stdout != "" || !strings.Contains(got.stderr, "must not be a symlink") {
		t.Errorf("exit code = %d, stdout = %q, stderr = %q", got.exitCode, got.stdout, got.stderr)
	}
}

func TestClearAcceptsNoArgumentsAndLeavesUninitializedProjectUntouched(t *testing.T) {
	t.Parallel()

	project := t.TempDir()

	got := runApplicationInDirectory(t, project, []string{"HOME=" + t.TempDir()}, "", "skills", "clear")

	if got.exitCode != 0 || got.stdout != "" || got.stderr != "" {
		t.Fatalf("exit code = %d, stdout = %q, stderr = %q; want silent success", got.exitCode, got.stdout, got.stderr)
	}
	if entries, err := os.ReadDir(project); err != nil || len(entries) != 0 {
		t.Errorf("uninitialized Project entries = %v, err = %v; want none", entries, err)
	}

	withArgument := runApplicationInDirectory(t, project, nil, "", "skills", "clear", "review")
	if withArgument.exitCode != 1 || withArgument.stdout != "" || withArgument.stderr == "" {
		t.Errorf("clear with argument: exit code = %d, stdout = %q, stderr = %q; want argument error", withArgument.exitCode, withArgument.stdout, withArgument.stderr)
	}
}

func TestClearLocksBeforeDecidingAnUninitializedProjectHasNothingToDo(t *testing.T) {
	project := t.TempDir()
	lockedDirectory, err := os.Open(project)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := lockedDirectory.Close(); err != nil {
			t.Errorf("close locked Project: %v", err)
		}
	})
	if err := syscall.Flock(int(lockedDirectory.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := syscall.Flock(int(lockedDirectory.Fd()), syscall.LOCK_UN); err != nil {
			t.Errorf("unlock Project: %v", err)
		}
	})

	got := runApplicationWithDependencies(t, project, nil, "", Dependencies{ProjectLockTimeout: 25 * time.Millisecond}, "skills", "clear")

	if got.exitCode != 1 || got.stdout != "" || !strings.Contains(got.stderr, "locked") {
		t.Errorf("exit code = %d, stdout = %q, stderr = %q; want lock timeout", got.exitCode, got.stdout, got.stderr)
	}
	if entries, err := os.ReadDir(project); err != nil || len(entries) != 0 {
		t.Errorf("uninitialized Project entries = %v, err = %v; want no created lock state", entries, err)
	}
}

func TestClearRejectsOrphanedTransactionStaging(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	stageDirectory := filepath.Join(project, ".agents", ".bond-stage-orphaned")
	if err := os.MkdirAll(stageDirectory, 0o755); err != nil {
		t.Fatal(err)
	}

	got := runApplicationInDirectory(t, project, nil, "", "skills", "clear")

	if got.exitCode != 1 || got.stdout != "" || !strings.Contains(got.stderr, "unresolved") {
		t.Errorf("exit code = %d, stdout = %q, stderr = %q; want unresolved-state error", got.exitCode, got.stdout, got.stderr)
	}
	if info, err := os.Stat(stageDirectory); err != nil || !info.IsDir() {
		t.Errorf("orphaned staging was changed: info = %v, err = %v", info, err)
	}
}

func TestClearRemovesOnlyManagedSkillsAndRetainsInitializedInfrastructure(t *testing.T) {
	t.Parallel()

	configDirectory := t.TempDir()
	for _, name := range []string{"linked", "copied"} {
		writeSkill(t, filepath.Join(configDirectory, "bond", "skills", name), name, name+" Skill")
	}
	project := t.TempDir()
	environment := []string{"XDG_CONFIG_HOME=" + configDirectory}
	if got := runApplicationInDirectory(t, project, environment, "", "skills", "add", "linked"); got.exitCode != 0 {
		t.Fatalf("add linked Skill: %q", got.stderr)
	}
	if got := runApplicationInDirectory(t, project, environment, "", "skills", "add", "copied", "--copy"); got.exitCode != 0 {
		t.Fatalf("add copied Skill: %q", got.stderr)
	}
	unmanaged := filepath.Join(project, ".agents", "skills", "unmanaged")
	writeSkill(t, unmanaged, "unmanaged", "User maintained")

	got := runApplicationInDirectory(t, project, environment, "", "skills", "clear")

	if got.exitCode != 0 || got.stdout != "" || got.stderr != "" {
		t.Fatalf("exit code = %d, stdout = %q, stderr = %q; want silent success", got.exitCode, got.stdout, got.stderr)
	}
	for _, name := range []string{"linked", "copied"} {
		if _, err := os.Lstat(filepath.Join(project, ".agents", "skills", name)); !os.IsNotExist(err) {
			t.Errorf("Managed Skill %q remains: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(unmanaged, "SKILL.md")); err != nil {
		t.Errorf("unmanaged Project Skill changed: %v", err)
	}
	manifestPath := filepath.Join(project, ".agents", "bond-manifest.json")
	wantManifest := "{\n  \"version\": 1,\n  \"skills\": []\n}\n"
	if contents, err := os.ReadFile(manifestPath); err != nil || string(contents) != wantManifest {
		t.Errorf("manifest = %q, err = %v; want empty versioned manifest", contents, err)
	}

	second := runApplicationInDirectory(t, project, environment, "", "skills", "clear")
	if second.exitCode != 0 || second.stdout != "" || second.stderr != "" {
		t.Errorf("clear initialized empty Project: exit code = %d, stdout = %q, stderr = %q", second.exitCode, second.stdout, second.stderr)
	}
	if _, err := os.Stat(filepath.Join(project, ".agents", "skills")); err != nil {
		t.Errorf("Project Skill directory was not retained: %v", err)
	}
}

func TestClearHandledFailureRestoresEveryManagedSkillAndManifest(t *testing.T) {
	t.Parallel()

	configDirectory := t.TempDir()
	for _, name := range []string{"review", "deploy"} {
		writeSkill(t, filepath.Join(configDirectory, "bond", "skills", name), name, name+" Skill")
	}
	project := t.TempDir()
	environment := []string{"XDG_CONFIG_HOME=" + configDirectory}
	if got := runApplicationInDirectory(t, project, environment, "", "skills", "add", "review", "deploy", "--copy"); got.exitCode != 0 {
		t.Fatalf("add Skills: %q", got.stderr)
	}
	manifestPath := filepath.Join(project, ".agents", "bond-manifest.json")
	before, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}

	got := runApplicationWithDependencies(t, project, environment, "", Dependencies{TransactionFailurePoint: afterFirstRemoval}, "skills", "clear")

	if got.exitCode != 1 || got.stdout != "" || got.stderr == "" {
		t.Fatalf("exit code = %d, stdout = %q, stderr = %q; want handled failure", got.exitCode, got.stdout, got.stderr)
	}
	for _, name := range []string{"review", "deploy"} {
		if _, err := os.Stat(filepath.Join(project, ".agents", "skills", name, "SKILL.md")); err != nil {
			t.Errorf("Managed Skill %q was not restored: %v", name, err)
		}
	}
	if after, err := os.ReadFile(manifestPath); err != nil || !bytes.Equal(after, before) {
		t.Errorf("manifest changed: before = %q, after = %q, err = %v", before, after, err)
	}
	if _, err := os.Lstat(filepath.Join(project, ".agents", "bond-journal.json")); !os.IsNotExist(err) {
		t.Errorf("transaction journal remains: %v", err)
	}
}

func TestClearRecoversInterruptionsAtEveryJournaledPhase(t *testing.T) {
	for _, test := range []struct {
		name              string
		interruptionPoint string
	}{
		{name: "journal written", interruptionPoint: afterJournalWrite},
		{name: "first removal", interruptionPoint: afterFirstRemoval},
		{name: "all removals", interruptionPoint: afterAllRemovals},
		{name: "manifest written", interruptionPoint: afterManifestWrite},
		{name: "staging removed", interruptionPoint: afterStageRemoval},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			configDirectory := t.TempDir()
			for _, name := range []string{"review", "deploy"} {
				writeSkill(t, filepath.Join(configDirectory, "bond", "skills", name), name, name+" Skill")
			}
			project := t.TempDir()
			environment := []string{"XDG_CONFIG_HOME=" + configDirectory}
			if got := runApplicationInDirectory(t, project, environment, "", "skills", "add", "review", "deploy"); got.exitCode != 0 {
				t.Fatalf("add Skills: %q", got.stderr)
			}

			interrupted := runApplicationWithDependencies(t, project, environment, "", Dependencies{TransactionInterruptionPoint: test.interruptionPoint}, "skills", "clear")
			if interrupted.exitCode != 1 || interrupted.stdout != "" || interrupted.stderr == "" {
				t.Fatalf("interrupted exit code = %d, stdout = %q, stderr = %q", interrupted.exitCode, interrupted.stdout, interrupted.stderr)
			}
			if _, err := os.Stat(filepath.Join(project, ".agents", "bond-journal.json")); err != nil {
				t.Fatalf("transaction journal was not retained: %v", err)
			}

			recovered := runApplicationInDirectory(t, project, environment, "", "skills", "clear")
			if recovered.exitCode != 0 || recovered.stdout != "" || recovered.stderr != "" {
				t.Fatalf("recovery exit code = %d, stdout = %q, stderr = %q", recovered.exitCode, recovered.stdout, recovered.stderr)
			}
			for _, name := range []string{"review", "deploy"} {
				if _, err := os.Lstat(filepath.Join(project, ".agents", "skills", name)); !os.IsNotExist(err) {
					t.Errorf("Managed Skill %q remains: %v", name, err)
				}
			}
			if _, err := os.Lstat(filepath.Join(project, ".agents", "bond-journal.json")); !os.IsNotExist(err) {
				t.Errorf("transaction journal remains after recovery: %v", err)
			}
		})
	}
}

func TestClearRejectsUnsafeStateWithoutDeletingManifestOwnedPaths(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	agentsDirectory := filepath.Join(project, ".agents")
	if err := os.Mkdir(agentsDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	outsideSkills := t.TempDir()
	writeSkill(t, filepath.Join(outsideSkills, "review"), "review", "Outside Project")
	if err := os.Symlink(outsideSkills, filepath.Join(agentsDirectory, "skills")); err != nil {
		t.Fatal(err)
	}
	manifest := projectManifest{Version: manifestVersion, Skills: []managedSkillRecord{{Name: "review", Source: "review", Mode: copyMode, Destination: ".agents/skills/review"}}}
	if err := writeProjectManifest(agentsDirectory, manifest); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(agentsDirectory, "bond-manifest.json")
	before, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}

	got := runApplicationInDirectory(t, project, nil, "", "skills", "clear")

	if got.exitCode != 1 || got.stdout != "" || !strings.Contains(got.stderr, "must not be a symlink") {
		t.Fatalf("exit code = %d, stdout = %q, stderr = %q", got.exitCode, got.stdout, got.stderr)
	}
	if _, err := os.Stat(filepath.Join(outsideSkills, "review", "SKILL.md")); err != nil {
		t.Errorf("unsafe destination target changed: %v", err)
	}
	if after, err := os.ReadFile(manifestPath); err != nil || !bytes.Equal(after, before) {
		t.Errorf("manifest changed: before = %q, after = %q, err = %v", before, after, err)
	}
	if matches, err := filepath.Glob(filepath.Join(agentsDirectory, ".bond-stage-*")); err != nil || len(matches) != 0 {
		t.Errorf("staging paths = %v, err = %v; want none", matches, err)
	}
	if _, err := os.Lstat(filepath.Join(agentsDirectory, "bond-journal.json")); !os.IsNotExist(err) {
		t.Errorf("journal was created: %v", err)
	}
}

func TestRemoveDeletesChangedLinkedAndCopiedManagedSkillsAndStaleOwnership(t *testing.T) {
	t.Parallel()

	configDirectory := t.TempDir()
	store := filepath.Join(configDirectory, "bond", "skills")
	for _, name := range []string{"linked", "copied", "stale"} {
		writeSkill(t, filepath.Join(store, name), name, name+" Skill")
	}
	project := t.TempDir()
	environment := []string{"XDG_CONFIG_HOME=" + configDirectory}
	if got := runApplicationInDirectory(t, project, environment, "", "skills", "add", "linked"); got.exitCode != 0 {
		t.Fatalf("add linked Skill: %q", got.stderr)
	}
	if got := runApplicationInDirectory(t, project, environment, "", "skills", "add", "copied", "--copy"); got.exitCode != 0 {
		t.Fatalf("add copied Skill: %q", got.stderr)
	}
	linkedDestination := filepath.Join(project, ".agents", "skills", "linked")
	if err := os.Remove(linkedDestination); err != nil {
		t.Fatal(err)
	}
	changedTarget := t.TempDir()
	if err := os.Symlink(changedTarget, linkedDestination); err != nil {
		t.Fatal(err)
	}
	copiedDestination := filepath.Join(project, ".agents", "skills", "copied")
	if err := os.WriteFile(filepath.Join(copiedDestination, "user-change"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(project, ".agents", "bond-manifest.json")
	manifest, err := readProjectManifest(filepath.Dir(manifestPath))
	if err != nil {
		t.Fatal(err)
	}
	manifest.Skills = append(manifest.Skills, managedSkillRecord{Name: "stale", Source: "stale", Mode: linkMode, Destination: ".agents/skills/stale"})
	if err := writeProjectManifest(filepath.Dir(manifestPath), manifest); err != nil {
		t.Fatal(err)
	}
	writeSkill(t, filepath.Join(project, ".agents", "skills", "unmanaged"), "unmanaged", "User maintained")

	got := runApplicationInDirectory(t, project, environment, "", "skills", "remove", "linked", "copied", "stale")

	if got.exitCode != 0 || got.stdout != "" || got.stderr != "" {
		t.Fatalf("exit code = %d, stdout = %q, stderr = %q; want silent success", got.exitCode, got.stdout, got.stderr)
	}
	for _, destination := range []string{linkedDestination, copiedDestination} {
		if _, err := os.Lstat(destination); !os.IsNotExist(err) {
			t.Errorf("Managed Skill destination remains at %q: %v", destination, err)
		}
	}
	if _, err := os.Stat(filepath.Join(project, ".agents", "skills", "unmanaged", "SKILL.md")); err != nil {
		t.Errorf("unmanaged Project Skill changed: %v", err)
	}
	contents, err := os.ReadFile(manifestPath)
	if err != nil || string(contents) != "{\n  \"version\": 1,\n  \"skills\": []\n}\n" {
		t.Errorf("manifest = %q, err = %v; want empty versioned manifest", contents, err)
	}
	if info, err := os.Stat(filepath.Join(project, ".agents", "skills")); err != nil || !info.IsDir() {
		t.Errorf("Project Skill directory was not retained: info = %v, err = %v", info, err)
	}
}

func TestRemoveRetainsProjectSkillDirectoryWhenFinalOwnershipWasAlreadyStale(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	agentsDirectory := filepath.Join(project, ".agents")
	if err := os.Mkdir(agentsDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := projectManifest{Version: manifestVersion, Skills: []managedSkillRecord{{Name: "review", Source: "review", Mode: linkMode, Destination: ".agents/skills/review"}}}
	if err := writeProjectManifest(agentsDirectory, manifest); err != nil {
		t.Fatal(err)
	}

	got := runApplicationInDirectory(t, project, []string{"HOME=" + t.TempDir()}, "", "skills", "remove", "review")

	if got.exitCode != 0 || got.stdout != "" || got.stderr != "" {
		t.Fatalf("exit code = %d, stdout = %q, stderr = %q", got.exitCode, got.stdout, got.stderr)
	}
	if info, err := os.Stat(filepath.Join(agentsDirectory, "skills")); err != nil || !info.IsDir() {
		t.Errorf("Project Skill directory was not retained: info = %v, err = %v", info, err)
	}
	contents, err := os.ReadFile(filepath.Join(agentsDirectory, "bond-manifest.json"))
	if err != nil || string(contents) != "{\n  \"version\": 1,\n  \"skills\": []\n}\n" {
		t.Errorf("manifest = %q, err = %v", contents, err)
	}
}

func TestRemoveReportsEveryInvalidOrUnmanagedNameInArgumentOrderWithoutMutation(t *testing.T) {
	t.Parallel()

	configDirectory := t.TempDir()
	for _, name := range []string{"review", "deploy"} {
		writeSkill(t, filepath.Join(configDirectory, "bond", "skills", name), name, name+" Skill")
	}
	project := t.TempDir()
	environment := []string{"XDG_CONFIG_HOME=" + configDirectory}
	if got := runApplicationInDirectory(t, project, environment, "", "skills", "add", "review", "deploy"); got.exitCode != 0 {
		t.Fatalf("add Skills: %q", got.stderr)
	}
	manifestPath := filepath.Join(project, ".agents", "bond-manifest.json")
	before, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}

	got := runApplicationInDirectory(t, project, environment, "", "skills", "remove", "missing", "Bad-Name", "review", "review", "also-missing")

	if got.exitCode != 1 || got.stdout != "" {
		t.Fatalf("exit code = %d, stdout = %q, stderr = %q", got.exitCode, got.stdout, got.stderr)
	}
	ordered := []string{"missing:", "Bad-Name:", "review:", "also-missing:"}
	position := -1
	for _, diagnostic := range ordered {
		next := strings.Index(got.stderr[position+1:], diagnostic)
		if next < 0 {
			t.Fatalf("stderr = %q, want ordered diagnostic %q", got.stderr, diagnostic)
		}
		position += next + 1
	}
	for _, name := range []string{"review", "deploy"} {
		if _, err := os.Lstat(filepath.Join(project, ".agents", "skills", name)); err != nil {
			t.Errorf("Managed Skill %q changed: %v", name, err)
		}
	}
	after, err := os.ReadFile(manifestPath)
	if err != nil || !bytes.Equal(after, before) {
		t.Errorf("manifest changed: before = %q, after = %q, err = %v", before, after, err)
	}
}

func TestRemoveHandledBatchFailureRestoresDestinationsAndManifest(t *testing.T) {
	t.Parallel()

	configDirectory := t.TempDir()
	for _, name := range []string{"review", "deploy"} {
		writeSkill(t, filepath.Join(configDirectory, "bond", "skills", name), name, name+" Skill")
	}
	project := t.TempDir()
	environment := []string{"XDG_CONFIG_HOME=" + configDirectory}
	if got := runApplicationInDirectory(t, project, environment, "", "skills", "add", "review", "deploy", "--copy"); got.exitCode != 0 {
		t.Fatalf("add Skills: %q", got.stderr)
	}
	manifestPath := filepath.Join(project, ".agents", "bond-manifest.json")
	before, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}

	got := runApplicationWithDependencies(t, project, environment, "", Dependencies{TransactionFailurePoint: afterFirstRemoval}, "skills", "remove", "review", "deploy")

	if got.exitCode != 1 || got.stdout != "" || got.stderr == "" {
		t.Fatalf("exit code = %d, stdout = %q, stderr = %q; want handled failure", got.exitCode, got.stdout, got.stderr)
	}
	for _, name := range []string{"review", "deploy"} {
		if _, err := os.Stat(filepath.Join(project, ".agents", "skills", name, "SKILL.md")); err != nil {
			t.Errorf("Managed Skill %q was not restored: %v", name, err)
		}
	}
	after, err := os.ReadFile(manifestPath)
	if err != nil || !bytes.Equal(after, before) {
		t.Errorf("manifest changed: before = %q, after = %q, err = %v", before, after, err)
	}
	if _, err := os.Lstat(filepath.Join(project, ".agents", "bond-journal.json")); !os.IsNotExist(err) {
		t.Errorf("transaction journal remains: %v", err)
	}
}

func TestRemoveRecoversInterruptionsAtEveryJournaledPhase(t *testing.T) {
	for _, test := range []struct {
		name                 string
		interruptionPoint    string
		interruptedCommitted bool
	}{
		{name: "journal written", interruptionPoint: afterJournalWrite},
		{name: "first removal", interruptionPoint: afterFirstRemoval},
		{name: "all removals", interruptionPoint: afterAllRemovals},
		{name: "manifest written", interruptionPoint: afterManifestWrite, interruptedCommitted: true},
		{name: "staging removed", interruptionPoint: afterStageRemoval, interruptedCommitted: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			configDirectory := t.TempDir()
			for _, name := range []string{"review", "deploy"} {
				writeSkill(t, filepath.Join(configDirectory, "bond", "skills", name), name, name+" Skill")
			}
			project := t.TempDir()
			environment := []string{"XDG_CONFIG_HOME=" + configDirectory}
			if got := runApplicationInDirectory(t, project, environment, "", "skills", "add", "review", "deploy"); got.exitCode != 0 {
				t.Fatalf("add Skills: %q", got.stderr)
			}

			interrupted := runApplicationWithDependencies(t, project, environment, "", Dependencies{TransactionInterruptionPoint: test.interruptionPoint}, "skills", "remove", "review", "deploy")
			if interrupted.exitCode != 1 || interrupted.stdout != "" || interrupted.stderr == "" {
				t.Fatalf("interrupted exit code = %d, stdout = %q, stderr = %q", interrupted.exitCode, interrupted.stdout, interrupted.stderr)
			}
			if _, err := os.Stat(filepath.Join(project, ".agents", "bond-journal.json")); err != nil {
				t.Fatalf("transaction journal was not retained: %v", err)
			}

			recovered := runApplicationInDirectory(t, project, environment, "", "skills", "remove", "review", "deploy")
			if test.interruptedCommitted {
				if recovered.exitCode != 1 || !strings.Contains(recovered.stderr, "no Managed Skill") {
					t.Fatalf("committed recovery exit code = %d, stderr = %q; want subsequent unmanaged validation", recovered.exitCode, recovered.stderr)
				}
			} else if recovered.exitCode != 0 || recovered.stdout != "" || recovered.stderr != "" {
				t.Fatalf("rolled-back recovery exit code = %d, stdout = %q, stderr = %q", recovered.exitCode, recovered.stdout, recovered.stderr)
			}
			for _, name := range []string{"review", "deploy"} {
				if _, err := os.Lstat(filepath.Join(project, ".agents", "skills", name)); !os.IsNotExist(err) {
					t.Errorf("removed Project Skill %q remains: %v", name, err)
				}
			}
			if _, err := os.Lstat(filepath.Join(project, ".agents", "bond-journal.json")); !os.IsNotExist(err) {
				t.Errorf("transaction journal remains after recovery: %v", err)
			}
		})
	}
}

func TestRemoveDoesNotRecoverInterruptedMutationWhenArgumentsFailValidation(t *testing.T) {
	t.Parallel()

	configDirectory := t.TempDir()
	for _, name := range []string{"review", "deploy"} {
		writeSkill(t, filepath.Join(configDirectory, "bond", "skills", name), name, name+" Skill")
	}
	project := t.TempDir()
	environment := []string{"XDG_CONFIG_HOME=" + configDirectory}
	if got := runApplicationInDirectory(t, project, environment, "", "skills", "add", "review", "deploy"); got.exitCode != 0 {
		t.Fatalf("add Skills: %q", got.stderr)
	}
	interrupted := runApplicationWithDependencies(t, project, environment, "", Dependencies{TransactionInterruptionPoint: afterFirstRemoval}, "skills", "remove", "review", "deploy")
	if interrupted.exitCode != 1 {
		t.Fatalf("interrupt removal: %q", interrupted.stderr)
	}
	reviewDestination := filepath.Join(project, ".agents", "skills", "review")
	if _, err := os.Lstat(reviewDestination); !os.IsNotExist(err) {
		t.Fatalf("first destination was not staged: %v", err)
	}
	manifestPath := filepath.Join(project, ".agents", "bond-manifest.json")
	before, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}

	got := runApplicationInDirectory(t, project, environment, "", "skills", "remove", "missing")

	if got.exitCode != 1 || got.stdout != "" || !strings.Contains(got.stderr, "no Managed Skill") {
		t.Fatalf("exit code = %d, stdout = %q, stderr = %q", got.exitCode, got.stdout, got.stderr)
	}
	if _, err := os.Lstat(reviewDestination); !os.IsNotExist(err) {
		t.Errorf("failed invocation recovered staged destination: %v", err)
	}
	after, err := os.ReadFile(manifestPath)
	if err != nil || !bytes.Equal(after, before) {
		t.Errorf("manifest changed: before = %q, after = %q, err = %v", before, after, err)
	}
	if _, err := os.Stat(filepath.Join(project, ".agents", "bond-journal.json")); err != nil {
		t.Errorf("journal was not preserved: %v", err)
	}
}

func TestRemoveRejectsSymlinkedInfrastructureWithoutTouchingItsTarget(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	agentsDirectory := filepath.Join(project, ".agents")
	if err := os.Mkdir(agentsDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	target := t.TempDir()
	marker := filepath.Join(target, "review")
	writeSkill(t, marker, "review", "Outside Project")
	if err := os.Symlink(target, filepath.Join(agentsDirectory, "skills")); err != nil {
		t.Fatal(err)
	}
	manifest := projectManifest{Version: manifestVersion, Skills: []managedSkillRecord{{Name: "review", Source: "review", Mode: copyMode, Destination: ".agents/skills/review"}}}
	if err := writeProjectManifest(agentsDirectory, manifest); err != nil {
		t.Fatal(err)
	}

	got := runApplicationInDirectory(t, project, []string{"HOME=" + t.TempDir()}, "", "skills", "remove", "review")

	if got.exitCode != 1 || got.stdout != "" || !strings.Contains(got.stderr, "must not be a symlink") {
		t.Fatalf("exit code = %d, stdout = %q, stderr = %q", got.exitCode, got.stdout, got.stderr)
	}
	if _, err := os.Stat(filepath.Join(marker, "SKILL.md")); err != nil {
		t.Errorf("symlink target changed: %v", err)
	}
}

func TestRemoveTimesOutOnConcurrentProjectMutation(t *testing.T) {
	project := t.TempDir()
	lockedDirectory, err := os.Open(project)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := lockedDirectory.Close(); err != nil {
			t.Errorf("close locked Project: %v", err)
		}
	})
	if err := syscall.Flock(int(lockedDirectory.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := syscall.Flock(int(lockedDirectory.Fd()), syscall.LOCK_UN); err != nil {
			t.Errorf("unlock Project: %v", err)
		}
	})

	got := runApplicationWithDependencies(t, project, nil, "", Dependencies{ProjectLockTimeout: 25 * time.Millisecond}, "skills", "remove", "review")

	if got.exitCode != 1 || got.stdout != "" || !strings.Contains(got.stderr, "locked") {
		t.Errorf("exit code = %d, stdout = %q, stderr = %q; want lock timeout", got.exitCode, got.stdout, got.stderr)
	}
}

func TestFailuresUsePlainStderrWithoutUsage(t *testing.T) {
	t.Parallel()

	got := runApplication(t, "--not-a-real-flag")

	if got.exitCode != 1 {
		t.Errorf("exit code = %d, want 1", got.exitCode)
	}
	if got.stdout != "" {
		t.Errorf("stdout = %q, want empty", got.stdout)
	}
	if got.stderr != "unknown flag: --not-a-real-flag\n" {
		t.Errorf("stderr = %q, want concise plain-text diagnostic", got.stderr)
	}
	for _, unwanted := range []string{"Usage:", "Error:", "goroutine", "github.com/"} {
		if strings.Contains(got.stderr, unwanted) {
			t.Errorf("stderr contains %q: %q", unwanted, got.stderr)
		}
	}
}
