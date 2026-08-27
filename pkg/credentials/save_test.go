package credentials_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pobear/pobear/pkg/credentials"
)

func TestSave(t *testing.T) {
	cases := []struct {
		name               string
		fileExists         bool
		initialFileContent string
		providerName       string
		credential         string
		secret             string
		wantFileContent    string
	}{
		{
			name:            "creates-new-file",
			fileExists:      false,
			providerName:    "acme",
			credential:      "token",
			secret:          "newsecret",
			wantFileContent: "POBEAR_PROVIDER_ACME_TOKEN=newsecret\n",
		},
		{
			name:            "empty-file",
			fileExists:      true,
			providerName:    "acme",
			credential:      "token",
			secret:          "newsecret",
			wantFileContent: "POBEAR_PROVIDER_ACME_TOKEN=newsecret\n",
		},
		{
			name:               "updates-existing-line-in-place",
			fileExists:         true,
			initialFileContent: "POBEAR_PROVIDER_ACME_TOKEN=oldsecret\nPOBEAR_PROVIDER_OTHER_TOKEN=untouched\n",
			providerName:       "acme",
			credential:         "token",
			secret:             "newsecret",
			wantFileContent:    "POBEAR_PROVIDER_ACME_TOKEN=newsecret\nPOBEAR_PROVIDER_OTHER_TOKEN=untouched\n",
		},
		{
			name:               "updates-one-credential-of-same-provider",
			fileExists:         true,
			initialFileContent: "POBEAR_PROVIDER_ACME_TOKEN=oldtoken\nPOBEAR_PROVIDER_ACME_USERNAME=untouched\n",
			providerName:       "acme",
			credential:         "token",
			secret:             "newtoken",
			wantFileContent:    "POBEAR_PROVIDER_ACME_TOKEN=newtoken\nPOBEAR_PROVIDER_ACME_USERNAME=untouched\n",
		},
		{
			name:               "appends-new-line",
			fileExists:         true,
			initialFileContent: "POBEAR_PROVIDER_OTHER_TOKEN=untouched\n",
			providerName:       "acme",
			credential:         "token",
			secret:             "newsecret",
			wantFileContent:    "POBEAR_PROVIDER_OTHER_TOKEN=untouched\nPOBEAR_PROVIDER_ACME_TOKEN=newsecret\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.wantFileContent == "" {
				t.Fatalf("test case %q: wantFileContent is empty", c.name)
			}
			if !c.fileExists && c.initialFileContent != "" {
				t.Fatalf("test case %q: initialFileContent is set but fileExists is false", c.name)
			}

			credPath := filepath.Join(t.TempDir(), "credentials")
			if c.fileExists {
				if err := os.WriteFile(credPath, []byte(c.initialFileContent), 0o600); err != nil {
					t.Fatalf("write fixture credentials file: %v", err)
				}
			}

			if err := credentials.Save(credPath, c.providerName, c.credential, c.secret); err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			got, err := os.ReadFile(credPath) //nolint:gosec // credPath is a t.TempDir() path this test constructed itself
			if err != nil {
				t.Fatalf("read resulting credentials file: %v", err)
			}
			if string(got) != c.wantFileContent {
				t.Fatalf("resulting file content =\n%q\nwant\n%q", got, c.wantFileContent)
			}

			info, err := os.Stat(credPath)
			if err != nil {
				t.Fatalf("stat resulting credentials file: %v", err)
			}
			if mode := info.Mode().Perm(); mode != 0o600 {
				t.Fatalf("resulting file mode = %v, want 0o600", mode)
			}
		})
	}
}
