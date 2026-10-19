package credentials_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Decodo/pobear/pkg/credentials"
)

func TestLoad(t *testing.T) {
	cases := []struct {
		name         string
		envVars      map[string]string
		fileExists   bool
		fileContent  string
		providerName string
		credential   string
		wantSecret   string
		wantOK       bool
		wantErr      bool
	}{
		{
			name:         "env-var-only",
			envVars:      map[string]string{"POBEAR_PROVIDER_ACME_TOKEN": "abc123"},
			fileExists:   true,
			providerName: "acme",
			credential:   "token",
			wantSecret:   "abc123",
			wantOK:       true,
		},
		{
			name:         "env-var-wins-over-file",
			envVars:      map[string]string{"POBEAR_PROVIDER_ACME_TOKEN": "envvalue"},
			fileExists:   true,
			fileContent:  "POBEAR_PROVIDER_ACME_TOKEN=filevalue\n",
			providerName: "acme",
			credential:   "token",
			wantSecret:   "envvalue",
			wantOK:       true,
		},
		{
			name:         "file-does-not-exist",
			fileExists:   false,
			providerName: "acme",
			credential:   "token",
		},
		{
			name:         "file-has-other-key-only",
			fileExists:   true,
			fileContent:  "POBEAR_PROVIDER_OTHER_TOKEN=nope\n",
			providerName: "acme",
			credential:   "token",
		},
		{
			name:         "file-multi-line-picks-right-key",
			fileExists:   true,
			fileContent:  "POBEAR_PROVIDER_OTHER_TOKEN=nope\nPOBEAR_PROVIDER_ACME_TOKEN=want\nPOBEAR_PROVIDER_ACME_USERNAME=nope-too\n",
			providerName: "acme",
			credential:   "token",
			wantSecret:   "want",
			wantOK:       true,
		},
		{
			name:         "file-only",
			fileExists:   true,
			fileContent:  "POBEAR_PROVIDER_ACME_TOKEN=filevalue\n",
			providerName: "acme",
			credential:   "token",
			wantSecret:   "filevalue",
			wantOK:       true,
		},
		{
			name:         "malformed-file-line",
			fileExists:   true,
			fileContent:  "bogus line without an equals sign\n",
			providerName: "acme",
			credential:   "token",
			wantErr:      true,
		},
		{
			name:         "malformed-line-does-not-block-a-different-lookup",
			fileExists:   true,
			fileContent:  "bogus line without an equals sign\nPOBEAR_PROVIDER_ACME_TOKEN=works\n",
			providerName: "acme",
			credential:   "token",
			wantSecret:   "works",
			wantOK:       true,
		},
		{
			name:         "neither-set",
			fileExists:   true,
			providerName: "acme",
			credential:   "token",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.wantErr && (c.wantSecret != "" || c.wantOK) {
				t.Fatalf("test case %q: wantErr is true but wantSecret/wantOK is also set", c.name)
			}

			for k, v := range c.envVars {
				t.Setenv(k, v)
			}

			credPath := filepath.Join(t.TempDir(), "credentials")
			if c.fileExists {
				if err := os.WriteFile(credPath, []byte(c.fileContent), 0o600); err != nil {
					t.Fatalf("write fixture credentials file: %v", err)
				}
			}

			secret, ok, err := credentials.Load(credPath, c.providerName, c.credential)

			if c.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if ok != c.wantOK {
				t.Fatalf("expected ok=%v, got %v", c.wantOK, ok)
			}
			if secret != c.wantSecret {
				t.Fatalf("expected secret %q, got %q", c.wantSecret, secret)
			}
		})
	}
}
