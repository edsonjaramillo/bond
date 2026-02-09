package commands

import (
	"fmt"
	"path/filepath"

	"bond/internal/config"
	"bond/internal/skills"
	"github.com/spf13/cobra"
)

// newUnlinkCmd builds the command that removes project skill symlinks.
func newUnlinkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unlink [skill ...]",
		Short: "Remove symlinked skills from ./.agents/skills",
		RunE:  runUnlink,
	}

	cmd.ValidArgsFunction = completeLinkedSkills
	return cmd
}

// runUnlink executes unlink operations and prints per-skill and summary status.
func runUnlink(cmd *cobra.Command, args []string) error {
	skillsDir, err := config.ProjectSkillsDir()
	if err != nil {
		return err
	}

	entries, err := resolveUnlinkTargets(skillsDir, args)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		// No arguments + no linked entries.
		fmt.Println("nothing to unlink")
		return nil
	}

	var unlinked, skipped, hardErrs int
	for _, entry := range entries {
		removed, err := skills.Unlink(entry.Path)
		if err != nil {
			hardErrs++
			fmt.Printf("error %s: %v\n", entry.Name, err)
			continue
		}
		if removed {
			unlinked++
			fmt.Printf("unlinked %s\n", entry.Name)
		} else {
			skipped++
			fmt.Printf("skipped %s (not a symlink)\n", entry.Name)
		}
	}

	// Missing/non-symlink targets are counted as skipped instead of hard errors.
	fmt.Printf("summary unlinked=%d skipped=%d errors=%d\n", unlinked, skipped, hardErrs)
	if hardErrs > 0 {
		return fmt.Errorf("unlink failed for %d skill(s)", hardErrs)
	}
	return nil
}

// resolveUnlinkTargets builds unlink candidates from args or discovered links.
func resolveUnlinkTargets(skillsDir string, args []string) ([]skills.Entry, error) {
	if len(args) == 0 {
		return skills.DiscoverLinked(skillsDir)
	}

	entries := make([]skills.Entry, 0, len(args))
	for _, name := range args {
		// Explicit args are accepted even if the path does not exist yet; Unlink
		// handles the missing path case and reports it as a no-op.
		entries = append(entries, skills.Entry{
			Name: name,
			Path: filepath.Join(skillsDir, name),
		})
	}
	return entries, nil
}

// completeLinkedSkills offers shell completions from currently linked skills.
func completeLinkedSkills(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	skillsDir, err := config.ProjectSkillsDir()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	linked, err := skills.DiscoverLinked(skillsDir)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	candidates := make([]string, 0, len(linked))
	for _, entry := range linked {
		candidates = append(candidates, entry.Name)
	}

	return candidates, cobra.ShellCompDirectiveNoFileComp
}
