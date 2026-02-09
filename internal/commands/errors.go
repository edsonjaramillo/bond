package commands

import "errors"

var errAlreadyReportedFailure = errors.New("command failed with details already reported")

// alreadyReportedFailure marks command failures whose user-facing errors were already printed.
func alreadyReportedFailure() error {
	return errAlreadyReportedFailure
}

// IsAlreadyReportedFailure reports whether an error is a command failure already shown to the user.
func IsAlreadyReportedFailure(err error) bool {
	return errors.Is(err, errAlreadyReportedFailure)
}
