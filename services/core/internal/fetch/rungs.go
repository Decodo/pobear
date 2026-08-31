package fetch

import "time"

func Backoff(attempt int) time.Duration {
	_ = attempt
	return 0
}
