package jobspec

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Mode int

const (
	ModePlain Mode = iota
	ModeRPC
)

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

func Parse(data []byte, mode Mode) (*Spec, error) {
	doc, verr := decodeSingleDocument(data)
	if verr != nil {
		return nil, verr
	}

	root, ok := doc.(map[string]any)
	if !ok {
		return nil, &ValidationError{Field: "$document", Message: "spec document must be a mapping"}
	}

	if err := checkUnknownKeys(root, "", "version", "source", "fetch", "limits", "stages", "sink"); err != nil {
		return nil, err
	}

	version, err := parseVersion(root)
	if err != nil {
		return nil, err
	}
	source, err := parseSource(root)
	if err != nil {
		return nil, err
	}
	fetch, err := parseFetch(root)
	if err != nil {
		return nil, err
	}
	limits, err := parseLimits(root)
	if err != nil {
		return nil, err
	}
	stages, err := parseStages(root, mode)
	if err != nil {
		return nil, err
	}
	sink, err := parseSink(root, mode)
	if err != nil {
		return nil, err
	}

	return &Spec{
		Version: version,
		Source:  source,
		Fetch:   fetch,
		Limits:  limits,
		Stages:  stages,
		Sink:    sink,
	}, nil
}

func decodeSingleDocument(data []byte) (any, *ValidationError) {
	dec := yaml.NewDecoder(bytes.NewReader(data))

	var first any
	if err := dec.Decode(&first); err != nil {
		if err == io.EOF {
			return nil, &ValidationError{Field: "$document", Message: "spec document must contain exactly one YAML document"}
		}
		return nil, &ValidationError{Field: "$document", Message: "spec document is not valid YAML: " + err.Error()}
	}

	var second any
	err := dec.Decode(&second)
	if err == nil {
		return nil, &ValidationError{Field: "$document", Message: "spec document must contain exactly one YAML document"}
	}
	if err != io.EOF {
		return nil, &ValidationError{Field: "$document", Message: "spec document is not valid YAML: " + err.Error()}
	}

	return first, nil
}

func vErr(field, format string, args ...any) *ValidationError {
	return &ValidationError{Field: field, Message: fmt.Sprintf(format, args...)}
}

func checkUnknownKeys(m map[string]any, prefix string, allowed ...string) *ValidationError {
	allowedSet := make(map[string]bool, len(allowed))
	for _, a := range allowed {
		allowedSet[a] = true
	}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		if allowedSet[k] {
			continue
		}
		field := k
		if prefix != "" {
			field = prefix + "." + k
		}
		return vErr(field, "unknown field `%s`", field)
	}
	return nil
}

func requireTier(present bool, field, use string, allowedTiers ...string) *ValidationError {
	if !present {
		return nil
	}
	for _, t := range allowedTiers {
		if use == t {
			return nil
		}
	}
	parts := make([]string, len(allowedTiers))
	for i, t := range allowedTiers {
		parts[i] = fmt.Sprintf("`fetch.use: %s`", t)
	}
	return vErr(field, "`%s` requires %s, got %q", field, strings.Join(parts, " or "), use)
}

// asMapping type-asserts raw (already looked up by the caller) as a mapping
// and rejects any key not in allowed. Every section of a spec (fetch, limits,
// source.crawl, ...) is "look the key up, is it required, then asMapping."
func asMapping(raw any, field string, allowed ...string) (map[string]any, *ValidationError) {
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, vErr(field, "`%s` must be a mapping", field)
	}
	if err := checkUnknownKeys(m, field, allowed...); err != nil {
		return nil, err
	}
	return m, nil
}

func getString(m map[string]any, key, def string) string {
	if v, ok := m[key]; ok {
		s, _ := v.(string)
		return s
	}
	return def
}

func getInt(m map[string]any, key string, def int) int {
	if v, ok := m[key]; ok {
		n, _ := v.(int)
		return n
	}
	return def
}

func getBool(m map[string]any, key string, def bool) bool {
	if v, ok := m[key]; ok {
		b, _ := v.(bool)
		return b
	}
	return def
}

// tierScopedString reads a fetch.* field that's only meaningful on some
// fetch tiers (fetch.name, fetch.template, fetch.proxy): rejected if set on
// a tier that doesn't support it, left at its zero value ("") if absent.
func tierScopedString(m map[string]any, key, field, use string, allowedTiers ...string) (string, *ValidationError) {
	raw, present := m[key]
	if err := requireTier(present, field, use, allowedTiers...); err != nil {
		return "", err
	}
	if !present {
		return "", nil
	}
	s, _ := raw.(string)
	return s, nil
}

func enumField(field, value string, allowed ...string) *ValidationError {
	for _, a := range allowed {
		if value == a {
			return nil
		}
	}
	return vErr(field, "`%s` must be one of %s, got %q", field, backtickList(allowed), value)
}

func backtickList(values []string) string {
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = "`" + v + "`"
	}
	return strings.Join(parts, ", ")
}

func validDuration(s string) bool {
	_, err := time.ParseDuration(s)
	return err == nil
}

func parseVersion(root map[string]any) (int, *ValidationError) {
	v, ok := root["version"]
	if !ok {
		return 0, vErr("version", "`version` is required")
	}
	n, ok := v.(int)
	if !ok || n != 1 {
		return 0, vErr("version", "`version` must be 1, got %v", v)
	}
	return n, nil
}
