package provider

import (
	"github.com/ohler55/ojg/jp"

	"github.com/Decodo/pobear/pkg/utils"
)

func parseResponse(root map[string]any) (Response, *ValidationError) {
	raw, ok := root["response"]
	if !ok {
		return Response{}, utils.Errorf("response", "`response` is required")
	}
	m, err := utils.AsMapping(raw, "response", "status", "body", "finalUrl")
	if err != nil {
		return Response{}, err
	}

	var status []string
	if v, ok := m["status"]; ok {
		paths, err := parseStatusPaths(v)
		if err != nil {
			return Response{}, err
		}
		status = paths
	}

	bodyRaw, hasBody := m["body"]
	if !hasBody {
		return Response{}, utils.Errorf("response.body", "`response.body` is required")
	}
	body, _ := bodyRaw.(string)
	if err := validateJSONPath(body, "response.body"); err != nil {
		return Response{}, err
	}

	var finalURL string
	if v, ok := m["finalUrl"]; ok {
		s, _ := v.(string)
		if err := validateJSONPath(s, "response.finalUrl"); err != nil {
			return Response{}, err
		}
		finalURL = s
	}

	return Response{Status: status, Body: body, FinalURL: finalURL}, nil
}

func parseStatusPaths(v any) ([]string, *ValidationError) {
	var raw []any
	switch t := v.(type) {
	case string:
		raw = []any{t}
	case []any:
		raw = t
	default:
		return nil, utils.Errorf("response.status", "`response.status` must be a JSONPath or a list of them")
	}
	if len(raw) == 0 {
		return nil, utils.Errorf("response.status", "`response.status` must not be an empty list")
	}
	paths := make([]string, len(raw))
	for i, p := range raw {
		s, _ := p.(string)
		if err := validateJSONPath(s, "response.status"); err != nil {
			return nil, err
		}
		paths[i] = s
	}
	return paths, nil
}

func validateJSONPath(s, field string) *ValidationError {
	if _, err := jp.ParseString(s); err != nil {
		return utils.Errorf(field, "`%s` is not a valid JSONPath: %q", field, s)
	}
	return nil
}
