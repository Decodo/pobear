package providerator

import (
	"io"

	"github.com/pobear/pobear/pkg/provider"
)

func DefaultCredentialPath() (string, error) {
	return "", ErrNotImplemented
}

func Auth(m *provider.Manifest, path string, prompt func(label string) (string, error), stdout io.Writer) error {
	_ = m
	_ = path
	_ = prompt
	_ = stdout
	return ErrNotImplemented
}

func CheckCredential(m *provider.Manifest, path string) (bool, error) {
	_ = m
	_ = path
	return false, ErrNotImplemented
}
