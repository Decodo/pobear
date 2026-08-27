package exitcode

import "github.com/pobear/pobear/pkg/fault"

const (
	Success        = 0
	AuthFailure    = 1
	UsageError     = 3
	PartialFailure = 4
	Fatal          = 5
)

func FromValidationFault(k fault.Kind) int {
	_ = k
	return -1
}

func FromRun(succeeded, failed int, kinds []fault.Kind) int {
	_ = succeeded
	_ = failed
	_ = kinds
	return -1
}
