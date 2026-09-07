package provider

import (
	"fmt"
	"regexp"
	"slices"

	"github.com/pobear/pobear/pkg/utils"
)

func parseOptions(root map[string]any, key, prefix string) ([]Option, *ValidationError) {
	raw, ok := root[key]
	if !ok {
		return []Option{}, nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, utils.Errorf(prefix+key, "`%s%s` must be a list", prefix, key)
	}

	options := make([]Option, 0, len(list))
	seen := make(map[string]int)
	for i, item := range list {
		field := fmt.Sprintf("%s%s[%d]", prefix, key, i)
		m, err := utils.AsMapping(item, field, "name", "type", "default", "pattern", "min", "max", "values")
		if err != nil {
			return nil, err
		}

		nameRaw, hasName := m["name"]
		if !hasName {
			return nil, utils.Errorf(field+".name", "`%s.name` is required", field)
		}
		name, _ := nameRaw.(string)
		if name == "url" {
			return nil, utils.Errorf(field+".name", "`%s.name` must not be `url` (reserved)", field)
		}
		if idx, dup := seen[name]; dup {
			return nil, utils.Errorf(field+".name", "`%s.name` duplicates `%s%s[%d].name`: %q", field, prefix, key, idx, name)
		}
		seen[name] = i

		typeRaw, hasType := m["type"]
		if !hasType {
			return nil, utils.Errorf(field+".type", "`%s.type` is required", field)
		}
		typ, _ := typeRaw.(string)
		if err := utils.EnumField(field+".type", typ, "bool", "string", "int", "enum"); err != nil {
			return nil, err
		}

		opt := Option{Name: name, Type: typ}

		if v, ok := m["pattern"]; ok {
			pattern, _ := v.(string)
			if _, err := regexp.Compile(pattern); err != nil {
				return nil, utils.Errorf(field+".pattern", "`%s.pattern` is not a valid regular expression: %q", field, pattern)
			}
			opt.Pattern = pattern
		}

		var min, max *int
		if v, ok := m["min"]; ok {
			n, _ := v.(int)
			min = &n
		}
		if v, ok := m["max"]; ok {
			n, _ := v.(int)
			max = &n
		}
		if min != nil && max != nil && *min > *max {
			return nil, utils.Errorf(field+".min", "`%s.min` must be <= `%s.max`, got min: %d, max: %d", field, field, *min, *max)
		}
		opt.Min = min
		opt.Max = max

		var values []string
		if v, ok := m["values"]; ok {
			list, _ := v.([]any)
			for _, item := range list {
				s, _ := item.(string)
				values = append(values, s)
			}
		}
		if typ == "enum" && len(values) == 0 {
			return nil, utils.Errorf(field+".values", "`%s.values` is required for an `enum` option", field)
		}
		opt.Values = values

		if v, hasDefault := m["default"]; hasDefault {
			if err := validateOptionDefault(v, typ, values, min, max, field); err != nil {
				return nil, err
			}
			opt.Default = v
		}

		options = append(options, opt)
	}
	return options, nil
}

func validateOptionDefault(v any, typ string, values []string, min, max *int, field string) *ValidationError {
	switch typ {
	case "enum":
		s, _ := v.(string)
		if !slices.Contains(values, s) {
			return utils.Errorf(field+".default", "`%s.default` must be one of %s, got %q", field, utils.BacktickList(values), s)
		}
	case "int":
		n, _ := v.(int)
		if (min != nil && n < *min) || (max != nil && n > *max) {
			return utils.Errorf(field+".default", "`%s.default` must be between `%s.min` and `%s.max`, got %d", field, field, field, n)
		}
	}
	return nil
}
