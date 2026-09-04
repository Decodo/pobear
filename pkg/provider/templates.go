package provider

import (
	"fmt"

	"github.com/pobear/pobear/pkg/utils"
)

func parseTemplates(root map[string]any) ([]Template, *ValidationError) {
	raw := root["templates"]
	list, ok := raw.([]any)
	if !ok {
		return nil, utils.Errorf("templates", "`templates` must be a list")
	}
	if len(list) == 0 {
		return nil, utils.Errorf("templates", "`templates` must have at least one entry")
	}

	templates := make([]Template, 0, len(list))
	seenNames := make(map[string]int)
	haveDefault := false
	for i, item := range list {
		field := fmt.Sprintf("templates[%d]", i)
		m, err := utils.AsMapping(item, field, "name", "default", "endpoint", "method", "options", "request")
		if err != nil {
			return nil, err
		}

		nameRaw, hasName := m["name"]
		if !hasName {
			return nil, utils.Errorf(field+".name", "`%s.name` is required", field)
		}
		name, _ := nameRaw.(string)
		if idx, dup := seenNames[name]; dup {
			return nil, utils.Errorf(field+".name", "`%s.name` duplicates `templates[%d].name`: %q", field, idx, name)
		}
		seenNames[name] = i

		isDefault := utils.GetBool(m, "default", false)
		if isDefault {
			if haveDefault {
				return nil, utils.Errorf(field+".default", "at most one of `templates[].default` may be `true`")
			}
			haveDefault = true
		}

		endpoint := utils.GetString(m, "endpoint", "")
		if _, hasEndpoint := m["endpoint"]; hasEndpoint && !utils.IsAbsoluteHTTPURL(endpoint) {
			return nil, utils.Errorf(field+".endpoint", "`%s.endpoint` must be an absolute `http`/`https` URL, got %q", field, endpoint)
		}

		method := utils.GetString(m, "method", "")

		options, err := parseOptions(m, "options", field+".")
		if err != nil {
			return nil, err
		}

		request, err := parseRequest(m, field+".", true)
		if err != nil {
			return nil, err
		}

		templates = append(templates, Template{
			Name:     name,
			Default:  isDefault,
			Endpoint: endpoint,
			Method:   method,
			Options:  options,
			Request:  request,
		})
	}
	return templates, nil
}
