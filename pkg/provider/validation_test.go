package provider_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/pobear/pobear/pkg/provider"
)

// fixture is the schema of every *.yaml file under testdata/validation,
// grouped into subdirectories by PROVIDER_REFERENCE.md section
// (structural/, auth/, options/, request/, response/, errors/,
// templates/, full-manifest/).
type fixture struct {
	Input        string             `yaml:"input"`
	WantErr      bool               `yaml:"wantErr"`
	WantField    string             `yaml:"wantField"`
	WantMessage  string             `yaml:"wantMessage"`
	WantManifest *provider.Manifest `yaml:"wantManifest"`
}

const validationDir = "testdata/validation"

func TestValidationFixtures(t *testing.T) {
	var files []string
	err := filepath.WalkDir(validationDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".yaml") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", validationDir, err)
	}
	if len(files) == 0 {
		t.Fatalf("no fixtures found under %s", validationDir)
	}

	for _, path := range files {
		rel, err := filepath.Rel(validationDir, path)
		if err != nil {
			t.Fatalf("relative path for %s: %v", path, err)
		}
		name := filepath.ToSlash(rel)
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(path) //nolint:gosec // path is discovered by walking the local testdata/validation tree, not user input
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}

			var fx fixture
			dec := yaml.NewDecoder(bytes.NewReader(raw))
			dec.KnownFields(true)
			if err := dec.Decode(&fx); err != nil {
				t.Fatalf("parse fixture %s: %v", path, err)
			}

			if fx.WantErr && fx.WantField == "" {
				t.Fatalf("fixture %s: wantErr is true but wantField is empty", path)
			}
			if !fx.WantErr && fx.WantField != "" {
				t.Fatalf("fixture %s: wantErr is false but wantField is set to %q", path, fx.WantField)
			}
			if fx.WantErr && fx.WantMessage == "" {
				t.Fatalf("fixture %s: wantErr is true but wantMessage is empty", path)
			}
			if !fx.WantErr && fx.WantMessage != "" {
				t.Fatalf("fixture %s: wantErr is false but wantMessage is set to %q", path, fx.WantMessage)
			}
			if fx.WantErr && fx.WantManifest != nil {
				t.Fatalf("fixture %s: wantErr is true but wantManifest is set", path)
			}

			manifest, err := provider.Parse([]byte(fx.Input))

			if fx.WantErr {
				var verr *provider.ValidationError
				if !errors.As(err, &verr) {
					t.Fatalf("expected a *provider.ValidationError, got %#v", err)
				}
				if verr.Field != fx.WantField {
					t.Fatalf("expected error field %q, got %q", fx.WantField, verr.Field)
				}
				if verr.Message != fx.WantMessage {
					t.Fatalf("expected error message %q, got %q", fx.WantMessage, verr.Message)
				}
				return
			}

			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if manifest == nil {
				t.Fatal("expected a non-nil *provider.Manifest, got nil")
			}
			if fx.WantManifest != nil && !reflect.DeepEqual(manifest, fx.WantManifest) {
				t.Fatalf("manifest mismatch\n got: %#v\nwant: %#v", manifest, fx.WantManifest)
			}
		})
	}
}
