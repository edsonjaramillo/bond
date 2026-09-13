package application

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func completeInstructionPaths(invocation Invocation) cobra.CompletionFunc {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		store, err := centralCollectionPath(invocation.Environment, instructionCollection)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		store, err = filepath.EvalSymlinks(store)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		entries, err := os.ReadDir(store)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		var candidates []string
		for _, entry := range entries {
			name := entry.Name()
			path := filepath.Join(store, name)
			info, statErr := os.Lstat(path)
			if statErr != nil || info.Mode()&os.ModeSymlink != 0 {
				continue
			}
			if info.Mode().IsRegular() {
				if components, parseErr := parseInstructionPath(name); parseErr == nil && len(components) == 1 && strings.HasPrefix(name, toComplete) {
					candidates = append(candidates, name)
				}
				continue
			}
			if !info.IsDir() || !validInstructionName(name) {
				continue
			}
			children, readErr := os.ReadDir(path)
			if readErr != nil {
				continue
			}
			for _, child := range children {
				candidate := name + "/" + child.Name()
				childInfo, childErr := os.Lstat(filepath.Join(path, child.Name()))
				if childErr != nil || childInfo.Mode()&os.ModeSymlink != 0 || !childInfo.Mode().IsRegular() || !strings.HasPrefix(candidate, toComplete) {
					continue
				}
				if components, parseErr := parseInstructionPath(candidate); parseErr == nil && len(components) == 2 {
					candidates = append(candidates, candidate)
				}
			}
		}
		sort.Strings(candidates)

		return candidates, cobra.ShellCompDirectiveNoFileComp
	}
}
