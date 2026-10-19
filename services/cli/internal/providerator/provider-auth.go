package providerator

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pobear/pobear/pkg/credentials"
	"github.com/pobear/pobear/pkg/provider"
)

func DefaultCredentialPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".pobear", "credentials"), nil
}

func Auth(m *provider.Manifest, path string, prompt func(label string) (string, error), stdout io.Writer) error {
	label := m.Name + " " + m.Auth.Credential
	secret, err := prompt(label)
	if err != nil {
		return err
	}
	if secret == "" {
		return fmt.Errorf("no secret entered; %s left unchanged", credentials.EnvVar(m.Name, m.Auth.Credential))
	}
	if err := credentials.Save(path, m.Name, m.Auth.Credential, secret); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "saved %s to %s\n", credentials.EnvVar(m.Name, m.Auth.Credential), path)
	return nil
}

func CheckCredential(m *provider.Manifest, path string) (bool, error) {
	_, ok, err := credentials.Load(path, m.Name, m.Auth.Credential)
	if err != nil {
		return false, err
	}
	return ok, nil
}
