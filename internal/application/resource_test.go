package application

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeStoredResourceFile(t *testing.T, store, resource, relative, contents string, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(store, resource, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), mode); err != nil {
		t.Fatal(err)
	}
	return path
}

func readManifestForTest(t *testing.T, project string) projectManifest {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join(project, ".agents", "bond-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest projectManifest
	if err := json.Unmarshal(contents, &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func TestBareResourcesPrintsHelp(t *testing.T) {
	t.Parallel()
	got := runApplication(t, "resources")
	if got.exitCode != 0 || !strings.Contains(got.stdout, "Usage:\n  bond resources") || got.stderr != "" {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", got.exitCode, got.stdout, got.stderr)
	}
}

func TestResourceStoreUsesPlatformUserConfigurationDirectoryWithoutXDG(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	home := t.TempDir()
	configDirectory := filepath.Join(home, ".config")
	if runtime.GOOS == "darwin" {
		configDirectory = filepath.Join(home, "Library", "Application Support")
	}
	store := filepath.Join(configDirectory, "bond", "resources")
	writeStoredResourceFile(t, store, "editor-config", ".editorconfig", "root = true\n", 0o644)

	got := runApplicationInDirectory(
		t,
		project,
		[]string{"XDG_CONFIG_HOME=", "HOME=" + home},
		"",
		"resources", "add", "editor-config", "--copy",
	)

	if got.exitCode != 0 || got.stdout != "" || got.stderr != "" {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", got.exitCode, got.stdout, got.stderr)
	}
	contents, err := os.ReadFile(filepath.Join(project, ".editorconfig"))
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "root = true\n" {
		t.Errorf("contents = %q, want %q", contents, "root = true\n")
	}
}

func TestResourcesAddLinksFilesAndRecordsManifestVersionTwo(t *testing.T) {
	t.Parallel()
	project := t.TempDir()
	config := t.TempDir()
	store := filepath.Join(config, "bond", "resources")
	source := writeStoredResourceFile(t, store, "editor-config", ".editorconfig", "root = true\n", 0o640)
	writeStoredResourceFile(t, store, "editor-config", "config/tool.json", "{}\n", 0o644)

	got := runApplicationInDirectory(t, project, []string{"XDG_CONFIG_HOME=" + config}, "", "resources", "add", "editor-config")
	if got.exitCode != 0 || got.stdout != "" || got.stderr != "" {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", got.exitCode, got.stdout, got.stderr)
	}
	target, err := os.Readlink(filepath.Join(project, ".editorconfig"))
	if err != nil || target != source {
		t.Fatalf("link target = %q, err = %v, want %q", target, err, source)
	}
	manifest := readManifestForTest(t, project)
	if manifest.Version != 2 || len(manifest.Resources) != 1 || len(manifest.Resources[0].Paths) != 2 {
		t.Fatalf("manifest = %#v", manifest)
	}
}

func TestResourcesAddCopyPreservesNewDirectoryAndFilePermissions(t *testing.T) {
	project := t.TempDir()
	config := t.TempDir()
	store := filepath.Join(config, "bond", "resources")
	writeStoredResourceFile(t, store, "hooks", "scripts/check.sh", "#!/bin/sh\n", 0o751)
	if err := os.Chmod(filepath.Join(store, "hooks", "scripts"), 0o711); err != nil {
		t.Fatal(err)
	}

	got := runApplicationInDirectory(t, project, []string{"XDG_CONFIG_HOME=" + config}, "", "resources", "add", "--copy", "hooks")
	if got.exitCode != 0 {
		t.Fatalf("exit = %d, stderr = %q", got.exitCode, got.stderr)
	}
	fileInfo, _ := os.Stat(filepath.Join(project, "scripts", "check.sh"))
	directoryInfo, _ := os.Stat(filepath.Join(project, "scripts"))
	if fileInfo.Mode().Perm() != 0o751 || directoryInfo.Mode().Perm() != 0o711 {
		t.Fatalf("file mode = %o, directory mode = %o", fileInfo.Mode().Perm(), directoryInfo.Mode().Perm())
	}
}

func TestResourcesRecreateSafeRelativeSymlinks(t *testing.T) {
	project := t.TempDir()
	config := t.TempDir()
	store := filepath.Join(config, "bond", "resources")
	writeStoredResourceFile(t, store, "config", "settings/base.json", "{}\n", 0o644)
	if err := os.Symlink("base.json", filepath.Join(store, "config", "settings", "current.json")); err != nil {
		t.Fatal(err)
	}
	got := runApplicationInDirectory(t, project, []string{"XDG_CONFIG_HOME=" + config}, "", "resources", "add", "config")
	if got.exitCode != 0 {
		t.Fatal(got.stderr)
	}
	target, err := os.Readlink(filepath.Join(project, "settings", "current.json"))
	if err != nil || target != "base.json" {
		t.Fatalf("relative symlink target = %q, err = %v", target, err)
	}
}

func TestResourceAndSkillOperationsShareManifestVersionTwo(t *testing.T) {
	project := t.TempDir()
	config := t.TempDir()
	resourceStore := filepath.Join(config, "bond", "resources")
	writeStoredResourceFile(t, resourceStore, "editor", ".editorconfig", "root = true\n", 0o644)
	writeSkill(t, filepath.Join(config, "bond", "skills", "review"), "review", "Review changes")
	environment := []string{"XDG_CONFIG_HOME=" + config}
	if got := runApplicationInDirectory(t, project, environment, "", "resources", "add", "editor"); got.exitCode != 0 {
		t.Fatal(got.stderr)
	}
	if got := runApplicationInDirectory(t, project, environment, "", "skills", "add", "review"); got.exitCode != 0 {
		t.Fatal(got.stderr)
	}
	if got := runApplicationInDirectory(t, project, environment, "", "skills", "remove", "review"); got.exitCode != 0 {
		t.Fatal(got.stderr)
	}
	manifest := readManifestForTest(t, project)
	if manifest.Version != 2 || len(manifest.Resources) != 1 || len(manifest.Skills) != 0 {
		t.Fatalf("manifest = %#v", manifest)
	}
}

func TestInterruptedResourceAddRollsBackBeforeNextMutation(t *testing.T) {
	project := t.TempDir()
	config := t.TempDir()
	store := filepath.Join(config, "bond", "resources")
	writeStoredResourceFile(t, store, "editor", ".editorconfig", "root = true\n", 0o644)
	environment := []string{"XDG_CONFIG_HOME=" + config}
	interrupted := runApplicationWithDependencies(t, project, environment, "", Dependencies{TransactionInterruptionPoint: afterFirstPublish}, "resources", "add", "editor")
	if interrupted.exitCode != 1 {
		t.Fatalf("interrupted exit = %d", interrupted.exitCode)
	}
	got := runApplicationInDirectory(t, project, environment, "", "resources", "add", "editor")
	if got.exitCode != 0 {
		t.Fatalf("recovery exit = %d, stderr = %q", got.exitCode, got.stderr)
	}
	if _, err := os.Lstat(filepath.Join(project, ".agents", "bond-journal.json")); !os.IsNotExist(err) {
		t.Fatalf("journal remains: %v", err)
	}
}

func TestInterruptedResourceRemoveRecoversBeforeRetry(t *testing.T) {
	project := t.TempDir()
	config := t.TempDir()
	store := filepath.Join(config, "bond", "resources")
	writeStoredResourceFile(t, store, "hooks", "scripts/check.sh", "#!/bin/sh\n", 0o755)
	writeStoredResourceFile(t, store, "hooks", "scripts/test.sh", "#!/bin/sh\n", 0o755)
	environment := []string{"XDG_CONFIG_HOME=" + config}
	if got := runApplicationInDirectory(t, project, environment, "", "resources", "add", "--copy", "hooks"); got.exitCode != 0 {
		t.Fatal(got.stderr)
	}
	interrupted := runApplicationWithDependencies(t, project, environment, "", Dependencies{TransactionInterruptionPoint: afterFirstRemoval}, "resources", "remove", "hooks")
	if interrupted.exitCode != 1 {
		t.Fatalf("interrupted exit = %d", interrupted.exitCode)
	}
	got := runApplicationInDirectory(t, project, environment, "", "resources", "remove", "hooks")
	if got.exitCode != 0 {
		t.Fatalf("recovery exit = %d, stderr = %q", got.exitCode, got.stderr)
	}
	if len(readManifestForTest(t, project).Resources) != 0 {
		t.Fatal("Resource ownership remains after recovered removal")
	}
}

func TestResourcesRemoveUsesManifestOwnershipAndRetainsDirectories(t *testing.T) {
	t.Parallel()
	project := t.TempDir()
	config := t.TempDir()
	store := filepath.Join(config, "bond", "resources")
	writeStoredResourceFile(t, store, "hooks", "scripts/check.sh", "#!/bin/sh\n", 0o755)
	environment := []string{"XDG_CONFIG_HOME=" + config}
	added := runApplicationInDirectory(t, project, environment, "", "resources", "add", "--copy", "hooks")
	if added.exitCode != 0 {
		t.Fatal(added.stderr)
	}
	if err := os.RemoveAll(filepath.Join(store, "hooks")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "scripts", "check.sh"), []byte("modified\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := runApplicationInDirectory(t, project, environment, "", "resources", "remove", "hooks")
	if got.exitCode != 0 || got.stdout != "" || got.stderr != "" {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", got.exitCode, got.stdout, got.stderr)
	}
	if _, err := os.Lstat(filepath.Join(project, "scripts", "check.sh")); !os.IsNotExist(err) {
		t.Fatalf("owned leaf remains: %v", err)
	}
	if info, err := os.Stat(filepath.Join(project, "scripts")); err != nil || !info.IsDir() {
		t.Fatalf("directory was not retained: %v", err)
	}
	manifest := readManifestForTest(t, project)
	if manifest.Version != 2 || len(manifest.Resources) != 0 {
		t.Fatalf("manifest = %#v", manifest)
	}
}

func TestResourcesAddRejectsUnsafeSourcesAndCollisionsAtomically(t *testing.T) {
	t.Parallel()
	project := t.TempDir()
	config := t.TempDir()
	store := filepath.Join(config, "bond", "resources")
	writeStoredResourceFile(t, store, "first", "shared.txt", "one", 0o644)
	writeStoredResourceFile(t, store, "second", "shared.txt", "two", 0o644)
	if err := os.MkdirAll(filepath.Join(store, "unsafe"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../outside", filepath.Join(store, "unsafe", "escape")); err != nil {
		t.Fatal(err)
	}

	got := runApplicationInDirectory(t, project, []string{"XDG_CONFIG_HOME=" + config}, "", "resources", "add", "first", "second", "unsafe")
	if got.exitCode != 1 || !strings.Contains(got.stderr, "collides") || !strings.Contains(got.stderr, "must not escape") {
		t.Fatalf("exit = %d, stderr = %q", got.exitCode, got.stderr)
	}
	if _, err := os.Lstat(filepath.Join(project, "shared.txt")); !os.IsNotExist(err) {
		t.Fatalf("partial installation exists: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(project, ".agents")); !os.IsNotExist(err) {
		t.Fatalf("Project infrastructure was created: %v", err)
	}
}

func TestResourcesRemoveRefusesOwnedLeafReplacedByDirectory(t *testing.T) {
	project := t.TempDir()
	config := t.TempDir()
	store := filepath.Join(config, "bond", "resources")
	writeStoredResourceFile(t, store, "editor", ".editorconfig", "root = true\n", 0o644)
	environment := []string{"XDG_CONFIG_HOME=" + config}
	if got := runApplicationInDirectory(t, project, environment, "", "resources", "add", "--copy", "editor"); got.exitCode != 0 {
		t.Fatal(got.stderr)
	}
	if err := os.Remove(filepath.Join(project, ".editorconfig")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(project, ".editorconfig"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := runApplicationInDirectory(t, project, environment, "", "resources", "remove", "editor")
	if got.exitCode != 1 || !strings.Contains(got.stderr, "refusing recursive removal") {
		t.Fatalf("exit = %d, stderr = %q", got.exitCode, got.stderr)
	}
	if len(readManifestForTest(t, project).Resources) != 1 {
		t.Fatal("ownership changed after refused removal")
	}
}

func TestSkillAddRejectsStaleManagedResourcePathOverlap(t *testing.T) {
	project := t.TempDir()
	config := t.TempDir()
	writeSkill(t, filepath.Join(config, "bond", "skills", "review"), "review", "Review changes")
	if err := os.MkdirAll(filepath.Join(project, ".agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := projectManifest{
		Version: manifestVersion2,
		Skills:  []managedSkillRecord{},
		Resources: []managedResourceRecord{{
			Name: "stale", Mode: copyMode, Paths: []string{".agents/skills/review/SKILL.md"},
		}},
	}
	if err := writeProjectManifest(filepath.Join(project, ".agents"), manifest); err != nil {
		t.Fatal(err)
	}

	got := runApplicationInDirectory(t, project, []string{"XDG_CONFIG_HOME=" + config}, "", "skills", "add", "review")
	if got.exitCode != 1 || !strings.Contains(got.stderr, "Managed Resource") {
		t.Fatalf("exit = %d, stderr = %q", got.exitCode, got.stderr)
	}
	if _, err := os.Lstat(filepath.Join(project, ".agents", "skills", "review")); !os.IsNotExist(err) {
		t.Fatalf("Skill destination changed: %v", err)
	}
}

func TestManifestRejectsCrossKindOwnershipOverlap(t *testing.T) {
	project := t.TempDir()
	agents := filepath.Join(project, ".agents")
	if err := os.MkdirAll(agents, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"version":2,"skills":[{"name":"review","source":"review","mode":"copy","destination":".agents/skills/review"}],"resources":[{"name":"config","mode":"copy","paths":[".agents/skills/review/SKILL.md"]}]}`
	if err := os.WriteFile(filepath.Join(agents, "bond-manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	got := runApplicationInDirectory(t, project, []string{"HOME=" + t.TempDir()}, "", "resources", "remove", "config")
	if got.exitCode != 1 || !strings.Contains(got.stderr, "overlaps Managed Skill") {
		t.Fatalf("exit = %d, stderr = %q", got.exitCode, got.stderr)
	}
}

func TestResourceAddPreservesDestinationCreatedAfterPreflight(t *testing.T) {
	project := t.TempDir()
	config := t.TempDir()
	store := filepath.Join(config, "bond", "resources")
	writeStoredResourceFile(t, store, "editor", ".editorconfig", "managed\n", 0o644)
	var hookCalls int
	dependencies := Dependencies{transactionHook: func(point string) error {
		if point != afterJournalWrite {
			return nil
		}
		hookCalls++
		return os.WriteFile(filepath.Join(project, ".editorconfig"), []byte("user\n"), 0o644)
	}}

	got := runApplicationWithDependencies(t, project, []string{"XDG_CONFIG_HOME=" + config}, "", dependencies, "resources", "add", "editor")
	if got.exitCode != 1 || hookCalls != 1 {
		t.Fatalf("exit = %d, hook calls = %d, stderr = %q", got.exitCode, hookCalls, got.stderr)
	}
	contents, err := os.ReadFile(filepath.Join(project, ".editorconfig"))
	if err != nil || string(contents) != "user\n" {
		t.Fatalf("post-preflight destination = %q, err = %v", contents, err)
	}
}

func TestResourceAddPreservesAncestorChangedAfterPreflight(t *testing.T) {
	project := t.TempDir()
	outside := t.TempDir()
	config := t.TempDir()
	store := filepath.Join(config, "bond", "resources")
	writeStoredResourceFile(t, store, "tool", "config/tool.json", "managed\n", 0o644)
	if err := os.Mkdir(filepath.Join(project, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	dependencies := Dependencies{transactionHook: func(point string) error {
		if point != afterJournalWrite {
			return nil
		}
		if err := os.Remove(filepath.Join(project, "config")); err != nil {
			return err
		}
		return os.Symlink(outside, filepath.Join(project, "config"))
	}}

	got := runApplicationWithDependencies(t, project, []string{"XDG_CONFIG_HOME=" + config}, "", dependencies, "resources", "add", "tool")
	if got.exitCode != 1 {
		t.Fatalf("exit = %d, stderr = %q", got.exitCode, got.stderr)
	}
	if target, err := os.Readlink(filepath.Join(project, "config")); err != nil || target != outside {
		t.Fatalf("post-preflight ancestor = %q, err = %v", target, err)
	}
	if _, err := os.Lstat(filepath.Join(outside, "tool.json")); !os.IsNotExist(err) {
		t.Fatalf("outside path changed: %v", err)
	}
}

func TestResourceRecoveryPreservesUnrecordedPlannedDirectory(t *testing.T) {
	project := t.TempDir()
	config := t.TempDir()
	store := filepath.Join(config, "bond", "resources")
	writeStoredResourceFile(t, store, "tool", "config/tool.json", "managed\n", 0o644)
	environment := []string{"XDG_CONFIG_HOME=" + config}
	interrupted := runApplicationWithDependencies(t, project, environment, "", Dependencies{TransactionInterruptionPoint: afterJournalWrite}, "resources", "add", "tool")
	if interrupted.exitCode != 1 {
		t.Fatalf("interrupted exit = %d", interrupted.exitCode)
	}
	if err := os.Mkdir(filepath.Join(project, "config"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := runApplicationInDirectory(t, project, environment, "", "resources", "add", "tool")
	if got.exitCode != 1 || !strings.Contains(got.stderr, "manual recovery") {
		t.Fatalf("exit = %d, stderr = %q", got.exitCode, got.stderr)
	}
	if info, err := os.Stat(filepath.Join(project, "config")); err != nil || !info.IsDir() {
		t.Fatalf("actor-created directory was not preserved: %v", err)
	}
}

func TestResourcesRejectCyclicSymlinkGraphs(t *testing.T) {
	for _, test := range []struct {
		name  string
		links map[string]string
	}{
		{name: "self reference", links: map[string]string{"self": "self"}},
		{name: "cycle", links: map[string]string{"first": "second", "second": "first"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			project := t.TempDir()
			config := t.TempDir()
			root := filepath.Join(config, "bond", "resources", "links")
			if err := os.MkdirAll(root, 0o755); err != nil {
				t.Fatal(err)
			}
			for path, target := range test.links {
				if err := os.Symlink(target, filepath.Join(root, path)); err != nil {
					t.Fatal(err)
				}
			}
			got := runApplicationInDirectory(t, project, []string{"XDG_CONFIG_HOME=" + config}, "", "resources", "add", "links")
			if got.exitCode != 1 || !strings.Contains(got.stderr, "cycle") {
				t.Fatalf("exit = %d, stderr = %q", got.exitCode, got.stderr)
			}
			if _, err := os.Lstat(filepath.Join(project, ".agents")); !os.IsNotExist(err) {
				t.Fatalf("Project changed: %v", err)
			}
		})
	}
}

func TestResourceArgumentCompletionUsesStoredAndManagedResourceNames(t *testing.T) {
	t.Parallel()
	project := t.TempDir()
	config := t.TempDir()
	store := filepath.Join(config, "bond", "resources")
	writeStoredResourceFile(t, store, "alpha", "a.txt", "a", 0o644)
	writeStoredResourceFile(t, store, "beta", "b.txt", "b", 0o644)
	environment := []string{"XDG_CONFIG_HOME=" + config}
	if got := runApplicationInDirectory(t, project, environment, "", "resources", "add", "alpha"); got.exitCode != 0 {
		t.Fatal(got.stderr)
	}
	add := runApplicationInDirectory(t, project, environment, "", "__complete", "resources", "add", "")
	remove := runApplicationInDirectory(t, project, environment, "", "__complete", "resources", "remove", "")
	if add.stdout != "beta\n:4\n" || remove.stdout != "alpha\n:4\n" {
		t.Fatalf("add completion = %q, remove completion = %q", add.stdout, remove.stdout)
	}
}
