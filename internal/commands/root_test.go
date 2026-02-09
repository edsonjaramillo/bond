package commands

import (
	"strings"
	"testing"
)

func TestRootRejectsInvalidColorFlagValue(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--color=invalid", "status"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), `invalid value for --color: "invalid"`) {
		t.Fatalf("error = %q, want invalid --color message", err.Error())
	}
}
