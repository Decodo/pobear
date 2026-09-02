package provider

import "fmt"

func parseTemplates(root map[string]any) ([]Template, *ValidationError) {
	raw := root["templates"]
	list, ok := raw.([]any)
	if !ok {
		return nil, vErr("templates", "`templates` must be a list")
	}
	if len(list) == 0 {
		return nil, vErr("templates", "`templates` must have at least one entry")
	}

	templates := make([]Template, 0, len(list))
	seenNames := make(map[string]int)
	haveDefault := false
	for i, item := range list {
		field := fmt.Sprintf("templates[%d]", i)
		m, ok := item.(map[string]any)
		if !ok {
			return nil, vErr(field, "`%s` must be a mapping", field)
		}
		if err := checkUnknownKeys(m, field, "name", "default", "endpoint", "method", "options", "request"); err != nil {
			return nil, err
		}

		nameRaw, hasName := m["name"]
		if !hasName {
			return nil, vErr(field+".name", "`%s.name` is required", field)
		}
		name, _ := nameRaw.(string)
		if idx, dup := seenNames[name]; dup {
			return nil, vErr(field+".name", "`%s.name` duplicates `templates[%d].name`: %q", field, idx, name)
		}
		seenNames[name] = i

		isDefault := false
		if v, ok := m["default"]; ok {
			b, _ := v.(bool)
			if b {
				if haveDefault {
					return nil, vErr(field+".default", "at most one of `templates[].default` may be `true`")
				}
				haveDefault = true
			}
			isDefault = b
		}

		endpoint := ""
		if v, ok := m["endpoint"]; ok {
			s, _ := v.(string)
			if !isAbsoluteHTTPURL(s) {
				return nil, vErr(field+".endpoint", "`%s.endpoint` must be an absolute `http`/`https` URL, got %q", field, s)
			}
			endpoint = s
		}

		method := ""
		if v, ok := m["method"]; ok {
			method, _ = v.(string)
		}

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
