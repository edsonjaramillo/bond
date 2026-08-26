package application

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/edsonjaramillo/bond/internal/skill"
	"github.com/spf13/cobra"
)

func completeSkillDraftOrganizations(invocation Invocation) cobra.CompletionFunc {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		directive := cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
		discovery, ok := completionStore(invocation.Environment)
		if !ok {
			return nil, directive
		}

		candidates := make([]string, 0, len(discovery.Organizations))
		for _, organization := range discovery.Organizations {
			candidate := organization + "/"
			if strings.HasPrefix(candidate, toComplete) {
				candidates = append(candidates, candidate)
			}
		}

		return candidates, directive
	}
}

func completeStoredSkills(invocation Invocation) cobra.CompletionFunc {
	return func(_ *cobra.Command, arguments []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		discovery, ok := completionStore(invocation.Environment)
		if !ok {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		unavailable, ok := completionUnavailableProjectSkillNames(invocation)
		if !ok {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		usedNames := make(map[string]bool, len(arguments))
		for _, argument := range arguments {
			if parsed, err := skill.ParseStoredPath(argument); err == nil {
				usedNames[parsed.Name] = true
			}
		}
		candidates := make([]string, 0, len(discovery.Paths))
		for _, storedPath := range discovery.Paths {
			parsed, err := skill.ParseStoredPath(storedPath)
			if err == nil && !unavailable[parsed.Name] && !usedNames[parsed.Name] && strings.HasPrefix(storedPath, toComplete) {
				candidates = append(candidates, storedPath)
			}
		}

		return candidates, cobra.ShellCompDirectiveNoFileComp
	}
}

func completeProjectSkills(invocation Invocation) cobra.CompletionFunc {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		names, ok := completionProjectSkillNames(invocation)
		if !ok {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		return candidatesWithPrefix(names, nil, toComplete), cobra.ShellCompDirectiveNoFileComp
	}
}

func completeManagedSkills(invocation Invocation) cobra.CompletionFunc {
	return func(_ *cobra.Command, arguments []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		manifest, ok := completionManifest(invocation)
		if !ok || !completionProjectSkillsInfrastructureIsSafe(invocation.WorkingDirectory) {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		names := make([]string, 0, len(manifest.Skills))
		for _, record := range manifest.Skills {
			names = append(names, record.Name)
		}
		sort.Strings(names)

		return candidatesWithPrefix(names, argumentSet(arguments), toComplete), cobra.ShellCompDirectiveNoFileComp
	}
}

func completionStore(environment []string) (skill.StoreDiscovery, bool) {
	store, err := storePath(environment)
	if err != nil {
		return skill.StoreDiscovery{}, false
	}
	discovery, err := skill.DiscoverStore(store)
	if os.IsNotExist(err) {
		return skill.StoreDiscovery{}, true
	}
	if err != nil {
		return skill.StoreDiscovery{}, false
	}

	return discovery, true
}

func completionProjectSkillNames(invocation Invocation) ([]string, bool) {
	discovery, exists, err := discoverProjectSkills(invocation)
	if err != nil || !exists {
		return nil, err == nil
	}

	return discovery.Names, true
}

func completionManifest(invocation Invocation) (projectManifest, bool) {
	if err := ensureNoInterruptedTransaction(invocation.WorkingDirectory); err != nil {
		return projectManifest{}, false
	}
	agentsDirectory := filepath.Join(invocation.WorkingDirectory, ".agents")
	exists, err := realDirectoryIfPresent(agentsDirectory, ".agents")
	if err != nil {
		return projectManifest{}, false
	}
	if !exists {
		return emptyProjectManifest(), true
	}
	manifest, err := readProjectManifest(agentsDirectory)
	if err != nil {
		return projectManifest{}, false
	}

	return manifest, true
}

func completionProjectSkillsInfrastructureIsSafe(project string) bool {
	agentsDirectory := filepath.Join(project, ".agents")
	agentsExist, err := realDirectoryIfPresent(agentsDirectory, ".agents")
	if err != nil || !agentsExist {
		return err == nil
	}
	_, err = realDirectoryIfPresent(filepath.Join(agentsDirectory, "skills"), ".agents/skills")

	return err == nil
}

func completionUnavailableProjectSkillNames(invocation Invocation) (map[string]bool, bool) {
	manifest, ok := completionManifest(invocation)
	if !ok {
		return nil, false
	}
	unavailable := make(map[string]bool, len(manifest.Skills))
	for _, record := range manifest.Skills {
		unavailable[record.Name] = true
	}

	agentsDirectory := filepath.Join(invocation.WorkingDirectory, ".agents")
	agentsExist, err := realDirectoryIfPresent(agentsDirectory, ".agents")
	if err != nil || !agentsExist {
		return unavailable, err == nil
	}
	skillsDirectory := filepath.Join(agentsDirectory, "skills")
	skillsExist, err := realDirectoryIfPresent(skillsDirectory, ".agents/skills")
	if err != nil || !skillsExist {
		return unavailable, err == nil
	}
	entries, err := os.ReadDir(skillsDirectory)
	if err != nil {
		return nil, false
	}
	for _, entry := range entries {
		unavailable[entry.Name()] = true
	}

	return unavailable, true
}

func argumentSet(arguments []string) map[string]bool {
	set := make(map[string]bool, len(arguments))
	for _, argument := range arguments {
		set[argument] = true
	}

	return set
}

func candidatesWithPrefix(names []string, excluded map[string]bool, prefix string) []string {
	var candidates []string
	for _, name := range names {
		if !excluded[name] && strings.HasPrefix(name, prefix) {
			candidates = append(candidates, name)
		}
	}

	return candidates
}
