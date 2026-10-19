package providerator_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Decodo/pobear/pkg/provider"
	"github.com/Decodo/pobear/services/cli/internal/providerator"
)

func TestLoadEmptyDirStillReturnsEmbeddedManifests(t *testing.T) {
	p := providerator.New(t.TempDir())

	got, err := p.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := providerator.Lookup(got, "decodo"); !ok {
		t.Fatalf("Load() = %v, want the embedded decodo manifest even with an empty providers directory", got)
	}
}

func TestLoadMissingDirStillReturnsEmbeddedManifests(t *testing.T) {
	p := providerator.New(filepath.Join(t.TempDir(), "does-not-exist"))

	got, err := p.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := providerator.Lookup(got, "decodo"); !ok {
		t.Fatalf("Load() = %v, want the embedded decodo manifest even with a missing providers directory", got)
	}
}

func TestLoadDiskManifestOverridesEmbeddedOfSameName(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "decodo.yaml"), `
name: decodo
endpoint: https://custom.example.com/scrape
auth: {type: bearer, credential: token}
request: {body: {url: "{{ .URL }}"}}
response: {body: "$.html"}
`)
	p := providerator.New(dir)

	got, err := p.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	m, ok := providerator.Lookup(got, "decodo")
	if !ok {
		t.Fatal("Load() has no decodo manifest")
	}
	if m.Endpoint != "https://custom.example.com/scrape" {
		t.Fatalf("decodo.Endpoint = %q, want the on-disk override, not the embedded default", m.Endpoint)
	}
}

func TestLoadOneManifest(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "decodo.yaml"), `
name: decodo
endpoint: https://data.decodo.com/v1/scrape
auth: {type: bearer, credential: token}
request: {body: {url: "{{ .URL }}"}}
response: {body: "$.html"}
`)
	p := providerator.New(dir)

	got, err := p.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 1 || got[0].Name != "decodo" {
		t.Fatalf("Load() = %v, want one manifest named decodo", got)
	}
}

func TestLookupFound(t *testing.T) {
	manifests := []*provider.Manifest{{Name: "acme"}, {Name: "decodo"}}

	got, ok := providerator.Lookup(manifests, "decodo")
	if !ok || got.Name != "decodo" {
		t.Fatalf("Lookup() = %v, %v, want decodo manifest, true", got, ok)
	}
}

func TestLookupNotFound(t *testing.T) {
	manifests := []*provider.Manifest{{Name: "acme"}}

	_, ok := providerator.Lookup(manifests, "bogus")
	if ok {
		t.Fatalf("Lookup() ok = true, want false")
	}
}

func TestResolveTemplateNoTemplatesUsesTopLevelOptions(t *testing.T) {
	m := &provider.Manifest{
		Name:    "acme",
		Options: []provider.Option{{Name: "device", Type: "string"}},
	}

	got, err := providerator.ResolveTemplate(m, "")
	if err != nil {
		t.Fatalf("ResolveTemplate: %v", err)
	}
	if len(got) != 1 || got[0].Name != "device" {
		t.Fatalf("ResolveTemplate() = %v, want [device]", got)
	}
}

func TestResolveTemplateDefaultTemplateUsedWhenOmitted(t *testing.T) {
	m := &provider.Manifest{
		Name: "decodo",
		Templates: []provider.Template{
			{Name: "universal", Default: true, Options: []provider.Option{{Name: "render", Type: "bool"}}},
			{Name: "amazon_product", Options: []provider.Option{{Name: "domain", Type: "enum"}}},
		},
	}

	got, err := providerator.ResolveTemplate(m, "")
	if err != nil {
		t.Fatalf("ResolveTemplate: %v", err)
	}
	if len(got) != 1 || got[0].Name != "render" {
		t.Fatalf("ResolveTemplate() = %v, want [render] (universal, the default)", got)
	}
}

func TestResolveTemplateRequiredWhenNoneDefault(t *testing.T) {
	m := &provider.Manifest{
		Name: "acme",
		Templates: []provider.Template{
			{Name: "product", Options: []provider.Option{{Name: "domain", Type: "enum"}}},
			{Name: "search", Options: []provider.Option{{Name: "query", Type: "string"}}},
		},
	}

	_, err := providerator.ResolveTemplate(m, "")
	if err == nil {
		t.Fatalf("expected an error (template required, none marked default), got nil")
	}
}

func TestResolveTemplateNamedSelection(t *testing.T) {
	m := &provider.Manifest{
		Name: "acme",
		Templates: []provider.Template{
			{Name: "universal", Default: true, Options: []provider.Option{{Name: "render", Type: "bool"}}},
			{Name: "amazon_product", Options: []provider.Option{{Name: "domain", Type: "enum"}}},
		},
	}

	got, err := providerator.ResolveTemplate(m, "amazon_product")
	if err != nil {
		t.Fatalf("ResolveTemplate: %v", err)
	}
	if len(got) != 1 || got[0].Name != "domain" {
		t.Fatalf("ResolveTemplate() = %v, want [domain]", got)
	}
}

func TestResolveTemplateUnknownNameErrors(t *testing.T) {
	m := &provider.Manifest{
		Name:      "acme",
		Templates: []provider.Template{{Name: "universal", Default: true}},
	}

	_, err := providerator.ResolveTemplate(m, "bogus")
	if err == nil {
		t.Fatalf("expected an error (unknown template name), got nil")
	}
}

func TestValidateOptionsBoolCoercion(t *testing.T) {
	declared := []provider.Option{{Name: "render", Type: "bool", Default: false}}

	got, err := providerator.ValidateOptions(declared, map[string]string{"render": "true"})
	if err != nil {
		t.Fatalf("ValidateOptions: %v", err)
	}
	if got["render"] != true {
		t.Fatalf("got[render] = %v (%T), want true (bool)", got["render"], got["render"])
	}
}

func TestValidateOptionsIntCoercionAndRange(t *testing.T) {
	minV, maxV := 0, 60000
	declared := []provider.Option{{Name: "maxWait", Type: "int", Min: &minV, Max: &maxV}}

	got, err := providerator.ValidateOptions(declared, map[string]string{"maxWait": "5000"})
	if err != nil {
		t.Fatalf("ValidateOptions: %v", err)
	}
	if got["maxWait"] != 5000 {
		t.Fatalf("got[maxWait] = %v, want 5000", got["maxWait"])
	}
}

func TestValidateOptionsIntOutOfRangeErrors(t *testing.T) {
	minV, maxV := 0, 60000
	declared := []provider.Option{{Name: "maxWait", Type: "int", Min: &minV, Max: &maxV}}

	_, err := providerator.ValidateOptions(declared, map[string]string{"maxWait": "70000"})
	if err == nil {
		t.Fatalf("expected an error (70000 > max 60000), got nil")
	}
}

func TestValidateOptionsPatternMismatchErrors(t *testing.T) {
	declared := []provider.Option{{Name: "geo", Type: "string", Pattern: "^[a-z]{2}$"}}

	_, err := providerator.ValidateOptions(declared, map[string]string{"geo": "USA"})
	if err == nil {
		t.Fatalf("expected an error (USA doesn't match ^[a-z]{2}$), got nil")
	}
}

func TestValidateOptionsEnumOutsideValuesErrors(t *testing.T) {
	declared := []provider.Option{{Name: "device", Type: "enum", Values: []string{"desktop", "mobile"}}}

	_, err := providerator.ValidateOptions(declared, map[string]string{"device": "tablet"})
	if err == nil {
		t.Fatalf("expected an error (tablet not in [desktop, mobile]), got nil")
	}
}

func TestValidateOptionsUnknownNameErrors(t *testing.T) {
	declared := []provider.Option{{Name: "render", Type: "bool"}}

	_, err := providerator.ValidateOptions(declared, map[string]string{"sessionId": "abc"})
	if err == nil {
		t.Fatalf("expected an error (sessionId not declared), got nil")
	}
}

func TestBuildSchemaWithTemplates(t *testing.T) {
	m := &provider.Manifest{
		Name: "decodo",
		Templates: []provider.Template{
			{Name: "universal", Default: true, Options: []provider.Option{
				{Name: "render", Type: "bool", Default: false},
				{Name: "geo", Type: "string", Pattern: "^[a-z]{2}$"},
			}},
			{Name: "amazon_product", Options: []provider.Option{
				{Name: "domain", Type: "enum", Values: []string{"com", "co.uk", "de"}, Default: "com"},
			}},
		},
	}

	got := providerator.BuildSchema(m)

	if got.Name != "decodo" {
		t.Fatalf("Name = %q, want decodo", got.Name)
	}
	if len(got.Templates) != 2 {
		t.Fatalf("len(Templates) = %d, want 2", len(got.Templates))
	}
	if !got.Templates[0].Default {
		t.Fatalf("Templates[0].Default = false, want true (universal)")
	}
	if len(got.Templates[0].Options) != 2 || got.Templates[0].Options[1].Pattern != "^[a-z]{2}$" {
		t.Fatalf("Templates[0].Options = %+v, want [render, geo] with geo's pattern set", got.Templates[0].Options)
	}
}

func TestBuildSchemaNoTemplates(t *testing.T) {
	m := &provider.Manifest{
		Name:    "acme",
		Options: []provider.Option{{Name: "device", Type: "string"}},
	}

	got := providerator.BuildSchema(m)

	if got.Name != "acme" {
		t.Fatalf("Name = %q, want acme", got.Name)
	}
	if got.Templates != nil {
		t.Fatalf("Templates = %v, want nil (no templates declared)", got.Templates)
	}
	if len(got.Options) != 1 || got.Options[0].Name != "device" {
		t.Fatalf("Options = %+v, want [device]", got.Options)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
