package provider

import (
	"fmt"
	"regexp"
	"slices"

	"github.com/Decodo/pobear/pkg/utils"
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

		name, err := parseOptionName(m, field, prefix, key, seen)
		if err != nil {
			return nil, err
		}
		seen[name] = i

		typ, err := parseOptionType(m, field)
		if err != nil {
			return nil, err
		}

		pattern, err := parseOptionPattern(m, field)
		if err != nil {
			return nil, err
		}

		min, max, err := parseOptionMinMax(m, field)
		if err != nil {
			return nil, err
		}

		values, err := parseOptionValues(m, field, typ)
		if err != nil {
			return nil, err
		}

		opt := Option{Name: name, Type: typ, Pattern: pattern, Min: min, Max: max, Values: values}

		if v, hasDefault := m["default"]; hasDefault {
			if err := validateOptionDefault(v, typ, values, pattern, min, max, field); err != nil {
				return nil, err
			}
			opt.Default = v
		}

		options = append(options, opt)
	}
	return options, nil
}

func parseOptionName(m map[string]any, field, prefix, key string, seen map[string]int) (string, *ValidationError) {
	name, err := requireUniqueName(m, field, prefix, key, seen)
	if err != nil {
		return "", err
	}
	if name == "url" {
		return "", utils.Errorf(field+".name", "`%s.name` must not be `url` (reserved)", field)
	}
	return name, nil
}

func parseOptionType(m map[string]any, field string) (string, *ValidationError) {
	typeRaw, hasType := m["type"]
	if !hasType {
		return "", utils.Errorf(field+".type", "`%s.type` is required", field)
	}
	typ, _ := typeRaw.(string)
	if err := utils.EnumField(field+".type", typ, "bool", "string", "int", "enum"); err != nil {
		return "", err
	}
	return typ, nil
}

func parseOptionPattern(m map[string]any, field string) (string, *ValidationError) {
	v, ok := m["pattern"]
	if !ok {
		return "", nil
	}
	pattern, _ := v.(string)
	if _, err := regexp.Compile(pattern); err != nil {
		return "", utils.Errorf(field+".pattern", "`%s.pattern` is not a valid regular expression: %q", field, pattern)
	}
	return pattern, nil
}

func parseOptionMinMax(m map[string]any, field string) (min, max *int, err *ValidationError) {
	if v, ok := m["min"]; ok {
		n, _ := v.(int)
		min = &n
	}
	if v, ok := m["max"]; ok {
		n, _ := v.(int)
		max = &n
	}
	if min != nil && max != nil && *min > *max {
		return nil, nil, utils.Errorf(field+".min", "`%s.min` must be <= `%s.max`, got min: %d, max: %d", field, field, *min, *max)
	}
	return min, max, nil
}

func parseOptionValues(m map[string]any, field, typ string) ([]string, *ValidationError) {
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
	return values, nil
}

func validateOptionDefault(v any, typ string, values []string, pattern string, min, max *int, field string) *ValidationError {
	switch typ {
	case "enum":
		s, ok := v.(string)
		if !ok {
			return utils.Errorf(field+".default", "`%s.default` must be one of %s, got %v", field, utils.BacktickList(values), v)
		}
		if !slices.Contains(values, s) {
			return utils.Errorf(field+".default", "`%s.default` must be one of %s, got %q", field, utils.BacktickList(values), s)
		}
	case "int":
		n, ok := v.(int)
		if !ok {
			return utils.Errorf(field+".default", "`%s.default` must be an int, got %v", field, v)
		}
		if (min != nil && n < *min) || (max != nil && n > *max) {
			return utils.Errorf(field+".default", "`%s.default` must be between `%s.min` and `%s.max`, got %d", field, field, field, n)
		}
	case "bool":
		if _, ok := v.(bool); !ok {
			return utils.Errorf(field+".default", "`%s.default` must be a bool, got %v", field, v)
		}
	case "string":
		s, ok := v.(string)
		if !ok {
			return utils.Errorf(field+".default", "`%s.default` must be a string, got %v", field, v)
		}
		if pattern != "" {
			matched, err := regexp.MatchString(pattern, s)
			if err != nil {
				return utils.Errorf(field+".default", "`%s.default` failed pattern check: %v", field, err)
			}
			if !matched {
				return utils.Errorf(field+".default", "`%s.default` must match `%s.pattern`, got %q", field, field, s)
			}
		}
	}
	return nil
}
