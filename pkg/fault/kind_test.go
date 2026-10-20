package fault_test

import (
	"errors"
	"testing"

	"github.com/Decodo/pobear/pkg/fault"
)

func TestKind(t *testing.T) {
	cases := []struct {
		kind             string
		wantValid        bool
		wantFetchOutcome bool
	}{
		{"auth", true, true},
		{"network", true, true},
		{"timeout", true, true},
		{"http", true, true},
		{"blocked", true, true},
		{"rateLimit", true, true},
		{"provider", true, true},
		{"spec", true, false},
		{"parse", true, false},
		{"plugin", true, false},
		{"bogus", false, false},
		{"", false, false},
		{"Auth", false, false},
		{"ratelimit", false, false},
		{" auth", false, false},
		{"authentication", false, false},
	}

	for _, c := range cases {
		t.Run(c.kind, func(t *testing.T) {
			if c.wantFetchOutcome && !c.wantValid {
				t.Fatalf("test case %q: wantFetchOutcome true requires wantValid true — a fetch-outcome kind is always valid", c.kind)
			}

			k := fault.Kind(c.kind)
			if got := fault.Valid(k); got != c.wantValid {
				t.Errorf("Valid(%q) = %v, want %v", c.kind, got, c.wantValid)
			}
			if got := fault.IsFetchOutcome(k); got != c.wantFetchOutcome {
				t.Errorf("IsFetchOutcome(%q) = %v, want %v", c.kind, got, c.wantFetchOutcome)
			}
		})
	}
}

func TestFormatFailure(t *testing.T) {
	err := errors.New("connection refused")

	if got, want := fault.FormatFailure("https://a.test", fault.Network, err), "https://a.test [network]: connection refused"; got != want {
		t.Errorf("FormatFailure with a kind = %q, want %q", got, want)
	}
	if got, want := fault.FormatFailure("https://a.test", "", err), "https://a.test: connection refused"; got != want {
		t.Errorf("FormatFailure with no kind = %q, want %q", got, want)
	}
}
