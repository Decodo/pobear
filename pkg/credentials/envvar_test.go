package credentials_test

import (
	"testing"

	"github.com/Decodo/pobear/pkg/credentials"
)

func TestEnvVar(t *testing.T) {
	cases := []struct {
		name         string
		providerName string
		credential   string
		want         string
	}{
		{name: "lowercase", providerName: "acme", credential: "token", want: "POBEAR_PROVIDER_ACME_TOKEN"},
		{name: "mixed-case", providerName: "Acme", credential: "Token", want: "POBEAR_PROVIDER_ACME_TOKEN"},
		{name: "already-uppercase", providerName: "DECODO", credential: "TOKEN", want: "POBEAR_PROVIDER_DECODO_TOKEN"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.want == "" {
				t.Fatalf("test case %q: want is empty", c.name)
			}

			got := credentials.EnvVar(c.providerName, c.credential)
			if got != c.want {
				t.Fatalf("EnvVar(%q, %q) = %q, want %q", c.providerName, c.credential, got, c.want)
			}
		})
	}
}
