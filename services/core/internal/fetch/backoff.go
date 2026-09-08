package fetch

import "time"

const maxBackoff = 5 * time.Second

func Backoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	d := 100 * time.Millisecond * time.Duration(1<<uint(attempt-1))
	if d > maxBackoff {
		return maxBackoff
	}
	return d
}
