package provider

import (
	"fmt"
	"text/template"
)

func parseRequest(root map[string]any, prefix string, required bool) (Request, *ValidationError) {
	raw, ok := root["request"]
	if !ok {
		if required {
			return Request{}, vErr(prefix+"request", "`%srequest` is required", prefix)
		}
		return Request{}, nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return Request{}, vErr(prefix+"request", "`%srequest` must be a mapping", prefix)
	}
	if err := checkUnknownKeys(m, prefix+"request", "body", "query"); err != nil {
		return Request{}, err
	}

	var body map[string]any
	if v, ok := m["body"]; ok {
		b, ok := v.(map[string]any)
		if !ok {
			return Request{}, vErr(prefix+"request.body", "`%srequest.body` must be a mapping", prefix)
		}
		if err := validateRequestBody(b, prefix); err != nil {
			return Request{}, err
		}
		body = b
	}

	var query map[string]any
	if v, ok := m["query"]; ok {
		q, ok := v.(map[string]any)
		if !ok {
			return Request{}, vErr(prefix+"request.query", "`%srequest.query` must be a mapping", prefix)
		}
		if err := validateRequestQuery(q, prefix); err != nil {
			return Request{}, err
		}
		query = q
	}

	return Request{Body: body, Query: query}, nil
}

func validateRequestBody(m map[string]any, prefix string) *ValidationError {
	for _, k := range sortedKeys(m) {
		v := m[k]
		field := fmt.Sprintf("%srequest.body.%s", prefix, k)
		if v == nil {
			return vErr(field, "`%s` must not be null", field)
		}
		if s, ok := v.(string); ok {
			if err := validateTemplateSyntax(s, field); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateRequestQuery(m map[string]any, prefix string) *ValidationError {
	for _, k := range sortedKeys(m) {
		v := m[k]
		field := fmt.Sprintf("%srequest.query.%s", prefix, k)
		switch t := v.(type) {
		case string:
			if err := validateTemplateSyntax(t, field); err != nil {
				return err
			}
		case bool, int, float64, nil:
		case []any:
			return vErr(field, "`%s` must be a scalar, got a list", field)
		case map[string]any:
			return vErr(field, "`%s` must be a scalar, got a map", field)
		default:
			return vErr(field, "`%s` must be a scalar, got %T", field, v)
		}
	}
	return nil
}

func validateTemplateSyntax(s, field string) *ValidationError {
	if _, err := template.New("").Parse(s); err != nil {
		return vErr(field, "`%s` is not a valid template: %q", field, s)
	}
	return nil
}
