package exitcode_test

import (
	"testing"

	"github.com/pobear/pobear/pkg/exitcode"
	"github.com/pobear/pobear/pkg/fault"
)

func TestFromValidationFault(t *testing.T) {
	cases := []struct {
		name string
		kind fault.Kind
		want int
	}{
		{name: "parse", kind: fault.Parse, want: exitcode.UsageError},
		{name: "spec", kind: fault.Spec, want: exitcode.UsageError},
		{name: "plugin", kind: fault.Plugin, want: exitcode.UsageError},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := exitcode.FromValidationFault(c.kind)
			if got != c.want {
				t.Fatalf("FromValidationFault(%q) = %d, want %d", c.kind, got, c.want)
			}
		})
	}
}
