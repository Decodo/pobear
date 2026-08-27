package exitcode

import "github.com/pobear/pobear/pkg/fault"

// Exit codes
const (
	Success        = 0
	AuthFailure    = 1
	UsageError     = 3
	PartialFailure = 4
	Fatal          = 5
)

// FromValidationFault maps a pre-run validation failure — a job spec or
// provider manifest that failed to parse, before any item ran — to an
// exit code.
func FromValidationFault(k fault.Kind) int {
	_ = k
	return -1
}

// FromRun maps a finished run's aggregate item outcome to an exit code.
func FromRun(succeeded, failed int, kinds []fault.Kind) int {
	_ = succeeded
	_ = failed
	_ = kinds
	return -1
}
