package provider

func parseAuth(root map[string]any) (Auth, *ValidationError) {
	raw, ok := root["auth"]
	if !ok {
		return Auth{}, vErr("auth", "`auth` is required")
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return Auth{}, vErr("auth", "`auth` must be a mapping")
	}
	if err := checkUnknownKeys(m, "auth", "type", "credential", "name"); err != nil {
		return Auth{}, err
	}

	typeRaw, hasType := m["type"]
	if !hasType {
		return Auth{}, vErr("auth.type", "`auth.type` is required")
	}
	t, _ := typeRaw.(string)
	if t != "basic" && t != "bearer" && t != "header" && t != "query" {
		return Auth{}, vErr("auth.type", "`auth.type` must be one of `basic`, `bearer`, `header`, `query`, got %q", t)
	}

	credRaw, hasCred := m["credential"]
	if !hasCred {
		return Auth{}, vErr("auth.credential", "`auth.credential` is required")
	}
	cred, _ := credRaw.(string)

	nameRaw, hasName := m["name"]
	name, _ := nameRaw.(string)
	if (t == "header" || t == "query") && !hasName {
		return Auth{}, vErr("auth.name", "`auth.name` is required when `auth.type` is `header` or `query`")
	}

	return Auth{Type: t, Credential: cred, Name: name}, nil
}
