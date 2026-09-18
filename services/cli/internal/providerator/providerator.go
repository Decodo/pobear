package providerator

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"

	"github.com/pobear/pobear/pkg/provider"
	"github.com/pobear/pobear/providers"
)

type Providerator struct {
	Dir string
}

func New(dir string) *Providerator {
	return &Providerator{Dir: dir}
}

func (p *Providerator) Load() ([]*provider.Manifest, error) {
	embedded, err := provider.LoadFS(providers.FS)
	if err != nil {
		return nil, err
	}
	if err := provider.ValidateUnique(embedded); err != nil {
		return nil, err
	}

	disk, err := provider.LoadDir(p.Dir)
	if err != nil {
		return nil, err
	}
	if err := provider.ValidateUnique(disk); err != nil {
		return nil, err
	}

	return mergeManifests(embedded, disk), nil
}

func mergeManifests(embedded, disk []*provider.Manifest) []*provider.Manifest {
	byName := make(map[string]*provider.Manifest, len(embedded)+len(disk))
	order := make([]string, 0, len(embedded)+len(disk))
	for _, m := range embedded {
		byName[m.Name] = m
		order = append(order, m.Name)
	}
	for _, m := range disk {
		if _, exists := byName[m.Name]; !exists {
			order = append(order, m.Name)
		}
		byName[m.Name] = m
	}
	merged := make([]*provider.Manifest, len(order))
	for i, name := range order {
		merged[i] = byName[name]
	}
	return merged
}

func Lookup(manifests []*provider.Manifest, name string) (*provider.Manifest, bool) {
	for _, m := range manifests {
		if m.Name == name {
			return m, true
		}
	}
	return nil, false
}

func ResolveTemplate(m *provider.Manifest, templateName string) ([]provider.Option, error) {
	rt, err := provider.ResolveTemplate(m, templateName)
	if err != nil {
		return nil, wrapTemplateSelectionError(err)
	}
	return rt.Options, nil
}

func wrapTemplateSelectionError(err error) error {
	switch e := err.(type) {
	case *provider.ErrTemplateNotFound:
		return fmt.Errorf("providerator: %s has no template %q", e.Manifest, e.Name)
	case *provider.ErrTemplateAmbiguous:
		return fmt.Errorf("providerator: %s has more than one template and none is marked default; --template is required", e.Manifest)
	default:
		return err
	}
}

func ValidateOptions(declared []provider.Option, opts map[string]string) (map[string]any, error) {
	byName := make(map[string]provider.Option, len(declared))
	for _, o := range declared {
		byName[o.Name] = o
	}

	result := make(map[string]any, len(opts))
	for name, raw := range opts {
		opt, ok := byName[name]
		if !ok {
			return nil, fmt.Errorf("providerator: no option %q", name)
		}
		v, err := coerceOption(opt, raw)
		if err != nil {
			return nil, err
		}
		result[name] = v
	}
	return result, nil
}

func coerceOption(opt provider.Option, raw string) (any, error) {
	switch opt.Type {
	case "bool":
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, fmt.Errorf("providerator: option %q must be a bool, got %q", opt.Name, raw)
		}
		return b, nil
	case "int":
		n, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("providerator: option %q must be an int, got %q", opt.Name, raw)
		}
		if opt.Min != nil && n < *opt.Min {
			return nil, fmt.Errorf("providerator: option %q must be >= %d, got %d", opt.Name, *opt.Min, n)
		}
		if opt.Max != nil && n > *opt.Max {
			return nil, fmt.Errorf("providerator: option %q must be <= %d, got %d", opt.Name, *opt.Max, n)
		}
		return n, nil
	case "enum":
		if !slices.Contains(opt.Values, raw) {
			return nil, fmt.Errorf("providerator: option %q must be one of %v, got %q", opt.Name, opt.Values, raw)
		}
		return raw, nil
	case "string":
		if opt.Pattern != "" {
			matched, err := regexp.MatchString(opt.Pattern, raw)
			if err != nil {
				return nil, err
			}
			if !matched {
				return nil, fmt.Errorf("providerator: option %q must match pattern %q, got %q", opt.Name, opt.Pattern, raw)
			}
		}
		return raw, nil
	default:
		return raw, nil
	}
}

type SchemaOption struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Pattern string   `json:"pattern,omitempty"`
	Min     *int     `json:"min,omitempty"`
	Max     *int     `json:"max,omitempty"`
	Values  []string `json:"values,omitempty"`
	Default any      `json:"default,omitempty"`
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
	s := Schema{Name: m.Name}
	if len(m.Templates) > 0 {
		s.Templates = make([]SchemaTemplate, len(m.Templates))
		for i, t := range m.Templates {
			s.Templates[i] = SchemaTemplate{Name: t.Name, Default: t.Default, Options: buildSchemaOptions(t.Options)}
		}
		return s
	}
	s.Options = buildSchemaOptions(m.Options)
	return s
}

func buildSchemaOptions(opts []provider.Option) []SchemaOption {
	out := make([]SchemaOption, len(opts))
	for i, o := range opts {
		out[i] = SchemaOption{
			Name:    o.Name,
			Type:    o.Type,
			Default: o.Default,
			Pattern: o.Pattern,
			Min:     o.Min,
			Max:     o.Max,
			Values:  o.Values,
		}
	}
	return out
}
