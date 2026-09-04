package tiers

import (
	"strconv"
	"testing"

	"github.com/pobear/pobear/pkg/fault"
)

func TestClassifyDefaultStatus(t *testing.T) {
	cases := []struct {
		status int
		want   fault.Kind
	}{
		{status: 200, want: ""},
		{status: 201, want: ""},
		{status: 299, want: ""},
		{status: 300, want: fault.HTTP},
		{status: 403, want: fault.Blocked},
		{status: 404, want: fault.HTTP},
		{status: 429, want: fault.RateLimit},
		{status: 500, want: fault.HTTP},
	}

	for _, c := range cases {
		t.Run(strconv.Itoa(c.status), func(t *testing.T) {
			got := classifyDefaultStatus(c.status)
			if got != c.want {
				t.Errorf("classifyDefaultStatus(%d) = %q, want %q", c.status, got, c.want)
			}
		})
	}
}
