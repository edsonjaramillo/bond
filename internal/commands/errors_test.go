package commands

import (
	"errors"
	"testing"
)

func TestIsAlreadyReportedFailure(t *testing.T) {
	if !IsAlreadyReportedFailure(alreadyReportedFailure()) {
		t.Fatal("IsAlreadyReportedFailure(alreadyReportedFailure()) = false, want true")
	}
	if IsAlreadyReportedFailure(errors.New("other")) {
		t.Fatal("IsAlreadyReportedFailure(other error) = true, want false")
	}
}
