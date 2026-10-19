package provider

import "github.com/Decodo/pobear/pkg/utils"

func requireUniqueName(m map[string]any, field, prefix, key string, seen map[string]int) (string, *ValidationError) {
	nameRaw, hasName := m["name"]
	if !hasName {
		return "", utils.Errorf(field+".name", "`%s.name` is required", field)
	}
	name, _ := nameRaw.(string)
	if idx, dup := seen[name]; dup {
		return "", utils.Errorf(field+".name", "`%s.name` duplicates `%s%s[%d].name`: %q", field, prefix, key, idx, name)
	}
	return name, nil
}
