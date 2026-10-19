package provider

import "github.com/pobear/pobear/pkg/utils"

func parseAuth(root map[string]any) (Auth, *ValidationError) {
	raw, ok := root["auth"]
	if !ok {
		return Auth{}, utils.Errorf("auth", "`auth` is required")
	}
	m, err := utils.AsMapping(raw, "auth", "type", "credential", "name")
	if err != nil {
		return Auth{}, err
	}

	typeRaw, hasType := m["type"]
	if !hasType {
		return Auth{}, utils.Errorf("auth.type", "`auth.type` is required")
	}
	t, _ := typeRaw.(string)
	if err := utils.EnumField("auth.type", t, "basic", "bearer", "header", "query"); err != nil {
		return Auth{}, err
	}

	credRaw, hasCred := m["credential"]
	if !hasCred {
		return Auth{}, utils.Errorf("auth.credential", "`auth.credential` is required")
	}
	cred, _ := credRaw.(string)

	_, hasName := m["name"]
	name := utils.GetString(m, "name", "")
	if (t == "header" || t == "query") && !hasName {
		return Auth{}, utils.Errorf("auth.name", "`auth.name` is required when `auth.type` is `header` or `query`")
	}

	return Auth{Type: t, Credential: cred, Name: name}, nil
}
