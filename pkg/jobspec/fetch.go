package jobspec

import (
	"fmt"
	"slices"
	"strings"

	"github.com/pobear/pobear/pkg/utils"
)

func parseFetch(root map[string]any) (Fetch, *ValidationError) {
	raw, ok := root["fetch"]
	if !ok {
		return Fetch{}, utils.Errorf("fetch", "`fetch` is required")
	}
	m, err := utils.AsMapping(raw, "fetch", "use", "name", "template", "proxy", "profile", "options", "timeout", "noRedirects", "headers")
	if err != nil {
		return Fetch{}, err
	}

	useRaw, hasUse := m["use"]
	if !hasUse {
		return Fetch{}, utils.Errorf("fetch.use", "`fetch.use` is required")
	}
	use, _ := useRaw.(string)
	if err := utils.EnumField("fetch.use", use, "local", "proxy", "provider"); err != nil {
		return Fetch{}, err
	}

	fetch := Fetch{Use: use}

	if fetch.Name, err = tierScopedString(m, "name", "fetch.name", use, "provider"); err != nil {
		return Fetch{}, err
	}
	if fetch.Template, err = tierScopedString(m, "template", "fetch.template", use, "provider"); err != nil {
		return Fetch{}, err
	}
	if fetch.Proxy, err = tierScopedString(m, "proxy", "fetch.proxy", use, "proxy"); err != nil {
		return Fetch{}, err
	}

	if err := parseFetchProfile(m, use, &fetch); err != nil {
		return Fetch{}, err
	}
	if err := parseFetchOptions(m, use, &fetch); err != nil {
		return Fetch{}, err
	}

	timeout := utils.GetString(m, "timeout", "30s")
	if !utils.ValidDuration(timeout) {
		return Fetch{}, utils.Errorf("fetch.timeout", "`fetch.timeout` is not a valid duration: %q", timeout)
	}
	fetch.Timeout = timeout

	fetch.NoRedirects, err = utils.GetBool(m, "noRedirects", "fetch.noRedirects", false)
	if err != nil {
		return Fetch{}, err
	}

	if err := parseFetchHeaders(m, &fetch); err != nil {
		return Fetch{}, err
	}

	return fetch, nil
}

func parseFetchProfile(m map[string]any, use string, fetch *Fetch) *ValidationError {
	_, present := m["profile"]
	if err := requireTier(present, "fetch.profile", use, "local", "proxy"); err != nil {
		return err
	}
	switch {
	case present:
		profile := utils.GetString(m, "profile", "")
		if err := utils.EnumField("fetch.profile", profile, "chrome", "firefox", "safari"); err != nil {
			return err
		}
		fetch.Profile = profile
	case use == "local" || use == "proxy":
		fetch.Profile = "chrome"
	}
	return nil
}

func parseFetchOptions(m map[string]any, use string, fetch *Fetch) *ValidationError {
	_, present := m["options"]
	if err := requireTier(present, "fetch.options", use, "provider"); err != nil {
		return err
	}
	switch {
	case present:
		optMap, ok := m["options"].(map[string]any)
		if !ok {
			return utils.Errorf("fetch.options", "`fetch.options` must be a mapping")
		}
		fetch.Options = optMap
	case use == "provider":
		fetch.Options = map[string]any{}
	}
	return nil
}

func parseFetchHeaders(m map[string]any, fetch *Fetch) *ValidationError {
	fetch.Headers = map[string]string{}
	v, ok := m["headers"]
	if !ok {
		return nil
	}
	hdrMap, ok := v.(map[string]any)
	if !ok {
		return utils.Errorf("fetch.headers", "`fetch.headers` must be a mapping")
	}
	headers := make(map[string]string, len(hdrMap))
	for k, val := range hdrMap {
		s, _ := val.(string)
		headers[k] = s
	}
	fetch.Headers = headers
	return nil
}

func requireTier(present bool, field, use string, allowedTiers ...string) *ValidationError {
	if !present {
		return nil
	}
	if slices.Contains(allowedTiers, use) {
		return nil
	}
	parts := make([]string, len(allowedTiers))
	for i, t := range allowedTiers {
		parts[i] = fmt.Sprintf("`fetch.use: %s`", t)
	}
	return utils.Errorf(field, "`%s` requires %s, got %q", field, strings.Join(parts, " or "), use)
}

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
