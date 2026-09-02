package providerator

import (
	"errors"

	"github.com/pobear/pobear/pkg/provider"
)

var ErrNotImplemented = errors.New("providerator: not implemented")

type Providerator struct {
	Dir string
}

func New(dir string) *Providerator {
	return &Providerator{Dir: dir}
}

func (p *Providerator) Load() ([]*provider.Manifest, error) {
	_ = p.Dir
	return nil, ErrNotImplemented
}

func Lookup(manifests []*provider.Manifest, name string) (*provider.Manifest, bool) {
	_ = manifests
	_ = name
	return nil, false
}

func ResolveTemplate(m *provider.Manifest, templateName string) ([]provider.Option, error) {
	_ = m
	_ = templateName
	return nil, ErrNotImplemented
}

func ValidateOptions(declared []provider.Option, opts map[string]string) (map[string]any, error) {
	_ = declared
	_ = opts
	return nil, ErrNotImplemented
}

type SchemaOption struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Default any      `json:"default,omitempty"`
	Pattern string   `json:"pattern,omitempty"`
	Min     *int     `json:"min,omitempty"`
	Max     *int     `json:"max,omitempty"`
	Values  []string `json:"values,omitempty"`
}

type SchemaTemplate struct {
	Name    string         `json:"name"`
	Default bool           `json:"default,omitempty"`
	Options []SchemaOption `json:"options"`
}

type Schema struct {
	Name      string           `json:"name"`
	Templates []SchemaTemplate `json:"templates,omitempty"`
	Options   []SchemaOption   `json:"options,omitempty"`
}

func BuildSchema(m *provider.Manifest) Schema {
	_ = m
	return Schema{}
}
