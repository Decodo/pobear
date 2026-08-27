package provider_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/pobear/pobear/pkg/provider"
)

// uniquenessFixture is the schema of every *.yaml file under
// testdata/uniqueness. Each Manifests entry is a raw manifest document,
// decoded directly (not through the still-stubbed Parse) into a
// *provider.Manifest, since this suite tests ValidateUnique in isolation.
type uniquenessFixture struct {
	Manifests   []string `yaml:"manifests"`
	WantField   string   `yaml:"wantField"`
	WantErr     bool     `yaml:"wantErr"`
	WantMessage string   `yaml:"wantMessage"`
}

const uniquenessDir = "testdata/uniqueness"

func TestUniqueNames(t *testing.T) {
	var files []string
	err := filepath.WalkDir(uniquenessDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".yaml") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", uniquenessDir, err)
	}
	if len(files) == 0 {
		t.Fatalf("no fixtures found under %s", uniquenessDir)
	}

	for _, path := range files {
		rel, err := filepath.Rel(uniquenessDir, path)
		if err != nil {
			t.Fatalf("relative path for %s: %v", path, err)
		}
		name := filepath.ToSlash(rel)
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(path) //nolint:gosec // path is discovered by walking the local testdata/uniqueness tree, not user input
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}

			var fx uniquenessFixture
			dec := yaml.NewDecoder(bytes.NewReader(raw))
			dec.KnownFields(true)
			if err := dec.Decode(&fx); err != nil {
				t.Fatalf("parse fixture %s: %v", path, err)
			}

			if fx.WantErr && fx.WantMessage == "" {
				t.Fatalf("fixture %s: wantErr is true but wantMessage is empty", path)
			}
			if !fx.WantErr && fx.WantMessage != "" {
				t.Fatalf("fixture %s: wantErr is false but wantMessage is set to %q", path, fx.WantMessage)
			}
			if fx.WantErr && fx.WantField == "" {
				t.Fatalf("fixture %s: wantErr is true but wantField is empty", path)
			}
			if !fx.WantErr && fx.WantField != "" {
				t.Fatalf("fixture %s: wantErr is false but wantField is set to %q", path, fx.WantField)
			}

			if len(fx.Manifests) == 0 {
				t.Fatalf("fixture %s: manifests is empty", path)
			}

			manifests := make([]*provider.Manifest, len(fx.Manifests))
			for i, doc := range fx.Manifests {
				var m provider.Manifest
				mdec := yaml.NewDecoder(bytes.NewReader([]byte(doc)))
				mdec.KnownFields(true)
				if err := mdec.Decode(&m); err != nil {
					t.Fatalf("fixture %s: decode manifests[%d]: %v", path, i, err)
				}
				manifests[i] = &m
			}

			err = provider.ValidateUnique(manifests)

			if fx.WantErr {
				var verr *provider.ValidationError
				if !errors.As(err, &verr) {
					t.Fatalf("expected a *provider.ValidationError, got %#v", err)
				}
				if verr.Message != fx.WantMessage {
					t.Fatalf("expected error message %q, got %q", fx.WantMessage, verr.Message)
				}
				if verr.Field != fx.WantField {
					t.Fatalf("expected error field %q, got %q", fx.WantField, verr.Field)
				}
				return
			}

			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}
