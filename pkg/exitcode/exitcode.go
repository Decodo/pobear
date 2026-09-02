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
	return UsageError
}

func FromRun(succeeded, failed int, kinds []fault.Kind) int {
	if failed == 0 {
		return Success
	}
	if succeeded > 0 {
		return PartialFailure
	}
	for _, k := range kinds {
		if k != fault.Auth {
			return Fatal
		}
	}
	return AuthFailure
}
