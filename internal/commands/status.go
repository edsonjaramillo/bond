package commands

import (
	"fmt"
	"sort"

	"bond/internal/config"
	"bond/internal/skills"
	"github.com/spf13/cobra"
)

// newStatusCmd builds the command that reports link health.
func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show project skill link status",
		Args:  cobra.NoArgs,
		RunE:  runStatus,
	}
}

// runStatus inspects project skill entries and prints a status summary.
func runStatus(cmd *cobra.Command, args []string) error {
	globalDir, err := config.GlobalSkillsDir()
	if err != nil {
		return err
	}

	projectDir, err := config.ProjectSkillsDir()
	if err != nil {
		return err
	}

	report, err := skills.InspectStatus(globalDir, projectDir)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "project %s\n", report.ProjectSkillsDir)
	fmt.Fprintf(out, "global %s\n", report.GlobalSkillsDir)

	entries := append([]skills.StatusEntry(nil), report.Entries...)
	sort.Slice(entries, func(i, j int) bool {
		ri := statusRank(entries[i].Status)
		rj := statusRank(entries[j].Status)
		if ri != rj {
			return ri < rj
		}
		return entries[i].Name < entries[j].Name
	})

	var linked, broken, external, conflict int
	for _, entry := range entries {
		fmt.Fprintf(out, "%s %s\n", entry.Status, entry.Name)
		switch entry.Status {
		case skills.StatusLinked:
			linked++
		case skills.StatusBroken:
			broken++
		case skills.StatusExternal:
			external++
		case skills.StatusConflict:
			conflict++
		}
	}

	fmt.Fprintf(out, "summary linked=%d broken=%d external=%d conflict=%d\n", linked, broken, external, conflict)
	return nil
}

func statusRank(status skills.StatusKind) int {
	switch status {
	case skills.StatusLinked:
		return 0
	case skills.StatusBroken:
		return 1
	case skills.StatusExternal:
		return 2
	default:
		return 3
	}
}
