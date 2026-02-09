package commands

import (
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

// runStatus inspects project skill entries and prints status details.
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

	if err := printOut(cmd, levelInfo, "project %s", report.ProjectSkillsDir); err != nil {
		return err
	}
	if err := printOut(cmd, levelInfo, "global %s", report.GlobalSkillsDir); err != nil {
		return err
	}

	entries := append([]skills.StatusEntry(nil), report.Entries...)
	sort.Slice(entries, func(i, j int) bool {
		ri := statusRank(entries[i].Status)
		rj := statusRank(entries[j].Status)
		if ri != rj {
			return ri < rj
		}
		return entries[i].Name < entries[j].Name
	})

	for _, entry := range entries {
		if err := printOut(cmd, statusLevel(entry.Status), "%s %s", entry.Status, entry.Name); err != nil {
			return err
		}
	}
	return nil
}

func statusLevel(status skills.StatusKind) string {
	switch status {
	case skills.StatusLinked:
		return levelOK
	case skills.StatusBroken:
		return levelError
	case skills.StatusExternal:
		return levelWarn
	default:
		return levelInfo
	}
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
