package utils

import (
	"fmt"
	"net/url"
	"slices"
	"sort"
	"strings"
	"time"
)

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

func Errorf(field, format string, args ...any) *ValidationError {
	return &ValidationError{Field: field, Message: fmt.Sprintf(format, args...)}
}

func SortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func CheckUnknownKeys(m map[string]any, prefix string, allowed ...string) *ValidationError {
	for _, k := range SortedKeys(m) {
		if slices.Contains(allowed, k) {
			continue
		}
		field := k
		if prefix != "" {
			field = prefix + "." + k
		}
		return Errorf(field, "unknown field `%s`", field)
	}
	return nil
}

func AsMapping(raw any, field string, allowed ...string) (map[string]any, *ValidationError) {
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, Errorf(field, "`%s` must be a mapping", field)
	}
	if err := CheckUnknownKeys(m, field, allowed...); err != nil {
		return nil, err
	}
	return m, nil
}

func GetString(m map[string]any, key, def string) string {
	if v, ok := m[key]; ok {
		s, _ := v.(string)
		return s
	}
	return def
}

func GetInt(m map[string]any, key string, def int) int {
	if v, ok := m[key]; ok {
		n, _ := v.(int)
		return n
	}
	return def
}

func GetBool(m map[string]any, key string, def bool) bool {
	if v, ok := m[key]; ok {
		b, _ := v.(bool)
		return b
	}
	return def
}

func EnumField(field, value string, allowed ...string) *ValidationError {
	if slices.Contains(allowed, value) {
		return nil
	}
	return Errorf(field, "`%s` must be one of %s, got %q", field, BacktickList(allowed), value)
}

func BacktickList(values []string) string {
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = "`" + v + "`"
	}
	return strings.Join(parts, ", ")
}

func ValidDuration(s string) bool {
	_, err := time.ParseDuration(s)
	return err == nil
}

func IsAbsoluteHTTPURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	return u.IsAbs() && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
