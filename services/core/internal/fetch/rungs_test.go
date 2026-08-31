package fetch_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/pobear/pobear/services/core/internal/fetch"
)

func TestBackoff(t *testing.T) {
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{attempt: 1, want: 100 * time.Millisecond},
		{attempt: 2, want: 200 * time.Millisecond},
		{attempt: 3, want: 400 * time.Millisecond},
		{attempt: 4, want: 800 * time.Millisecond},
		{attempt: 5, want: 1600 * time.Millisecond},
		{attempt: 6, want: 3200 * time.Millisecond},
		{attempt: 7, want: 5 * time.Second},
		{attempt: 8, want: 5 * time.Second},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("attempt=%d", c.attempt), func(t *testing.T) {
			got := fetch.Backoff(c.attempt)
			if got != c.want {
				t.Errorf("Backoff(%d) = %v, want %v", c.attempt, got, c.want)
			}
		})
	}
}
