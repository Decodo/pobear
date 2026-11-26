package tiers

import (
	"net/http"
	"strings"
	"testing"
)

type headerContains struct {
	name, substr string
}

func TestProfileHeadersMatchTLSVersion(t *testing.T) {
	cases := []struct {
		profile string
		want    func(version string) []headerContains
	}{
		{"chrome", func(v string) []headerContains {
			return []headerContains{
				{"User-Agent", "Chrome/" + v + ".0.0.0 "},
				{"Sec-Ch-Ua", `"Google Chrome";v="` + v + `"`},
				{"Sec-Ch-Ua", `"Chromium";v="` + v + `"`},
			}
		}},
		{"firefox", func(v string) []headerContains {
			return []headerContains{
				{"User-Agent", "rv:" + v + ".0)"},
				{"User-Agent", "Firefox/" + v + ".0"},
			}
		}},
		{"safari", func(v string) []headerContains {
			return []headerContains{
				{"User-Agent", "Version/" + v + " Safari/"},
			}
		}},
	}
	for _, c := range cases {
		t.Run(c.profile, func(t *testing.T) {
			helloID, err := helloIDForProfile(c.profile)
			if err != nil {
				t.Fatalf("helloIDForProfile(%q): %v", c.profile, err)
			}
			headers, ok := profileHeaders[c.profile]
			if !ok {
				t.Fatalf("no default headers for profile %q", c.profile)
			}
			for _, w := range c.want(helloID.Version) {
				if got := headers[w.name]; !strings.Contains(got, w.substr) {
					t.Errorf("%s = %q, want it to contain %q (TLS preset version %s)", w.name, got, w.substr, helloID.Version)
				}
			}
		})
	}
}

func TestProfileHeadersNeverSetAcceptEncoding(t *testing.T) {
	for profile, headers := range profileHeaders {
		for name := range headers {
			if http.CanonicalHeaderKey(name) == "Accept-Encoding" {
				t.Errorf("profile %q sets %s; that turns off Go's automatic gzip decoding", profile, name)
			}
		}
	}
}
