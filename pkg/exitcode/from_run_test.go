package exitcode_test

import (
	"testing"

	"github.com/pobear/pobear/pkg/exitcode"
	"github.com/pobear/pobear/pkg/fault"
)

func TestFromRun(t *testing.T) {
	cases := []struct {
		name      string
		succeeded int
		failed    int
		kinds     []fault.Kind
		want      int
	}{
		{name: "all-succeeded", succeeded: 5, failed: 0, kinds: nil, want: exitcode.Success},
		{name: "zero-items", succeeded: 0, failed: 0, kinds: nil, want: exitcode.Success},
		{name: "total-failure-all-auth", succeeded: 0, failed: 3, kinds: []fault.Kind{fault.Auth, fault.Auth, fault.Auth}, want: exitcode.AuthFailure},
		{name: "total-failure-all-network", succeeded: 0, failed: 3, kinds: []fault.Kind{fault.Network, fault.Network, fault.Network}, want: exitcode.Fatal},
		{name: "total-failure-mixed-with-auth", succeeded: 0, failed: 2, kinds: []fault.Kind{fault.Auth, fault.Network}, want: exitcode.Fatal},
		{name: "total-failure-auth-last", succeeded: 0, failed: 2, kinds: []fault.Kind{fault.Network, fault.Auth}, want: exitcode.Fatal},
		{name: "total-failure-unknown-kind", succeeded: 0, failed: 1, kinds: []fault.Kind{"authentication"}, want: exitcode.Fatal},
		{name: "total-failure-all-unclassified", succeeded: 0, failed: 2, kinds: nil, want: exitcode.Fatal},
		{name: "total-failure-partially-unclassified", succeeded: 0, failed: 2, kinds: []fault.Kind{fault.Auth}, want: exitcode.Fatal},
		{name: "partial-failure-auth-only", succeeded: 2, failed: 1, kinds: []fault.Kind{fault.Auth}, want: exitcode.PartialFailure},
		{name: "partial-failure-other-kind", succeeded: 2, failed: 1, kinds: []fault.Kind{fault.Network}, want: exitcode.PartialFailure},
		{name: "partial-failure-mixed", succeeded: 3, failed: 2, kinds: []fault.Kind{fault.Auth, fault.Timeout}, want: exitcode.PartialFailure},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if len(c.kinds) > c.failed {
				t.Fatalf("test case %q: len(kinds) = %d, failed = %d — kinds can't exceed failed", c.name, len(c.kinds), c.failed)
			}

			got := exitcode.FromRun(c.succeeded, c.failed, c.kinds)
			if got != c.want {
				t.Fatalf("FromRun(%d, %d, %v) = %d, want %d", c.succeeded, c.failed, c.kinds, got, c.want)
			}
		})
	}
}
