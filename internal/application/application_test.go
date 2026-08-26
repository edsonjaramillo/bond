package application

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
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

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run(context.Background(), Invocation{
		Arguments:        arguments,
		Environment:      environment,
		WorkingDirectory: directory,
		Stdin:            strings.NewReader(stdin),
		Stdout:           &stdout,
		Stderr:           &stderr,
	}, Dependencies{})

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
