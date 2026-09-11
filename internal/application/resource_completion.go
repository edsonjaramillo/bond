package application

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func completeStoredResources(invocation Invocation) cobra.CompletionFunc {
	return func(_ *cobra.Command, arguments []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		store, err := resourceStorePath(invocation.Environment)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		entries, err := os.ReadDir(store)
		if os.IsNotExist(err) {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		manifest, ok := completionManifest(invocation)
		if !ok {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		excluded := argumentSet(arguments)
		for _, record := range manifest.Resources {
			excluded[record.Name] = true
		}
		var candidates []string
		for _, entry := range entries {
			name := entry.Name()
			if excluded[name] || !strings.HasPrefix(name, toComplete) || !validResourceName(name) {
				continue
			}
			info, err := os.Lstat(filepath.Join(store, name))
			if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				continue
			}
			if _, err := inspectStoredResource(invocation.Environment, name); err == nil {
				candidates = append(candidates, name)
			}
		}
		sort.Strings(candidates)

		return candidates, cobra.ShellCompDirectiveNoFileComp
	}
}

func completeManagedResources(invocation Invocation) cobra.CompletionFunc {
	return func(_ *cobra.Command, arguments []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		manifest, ok := completionManifest(invocation)
		if !ok {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		names := make([]string, 0, len(manifest.Resources))
		for _, record := range manifest.Resources {
			names = append(names, record.Name)
		}
		sort.Strings(names)

		return candidatesWithPrefix(names, argumentSet(arguments), toComplete), cobra.ShellCompDirectiveNoFileComp
	}
}
