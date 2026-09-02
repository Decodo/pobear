package jobspec

func parseFetch(root map[string]any) (Fetch, *ValidationError) {
	raw, ok := root["fetch"]
	if !ok {
		return Fetch{}, vErr("fetch", "`fetch` is required")
	}
	m, err := asMapping(raw, "fetch", "use", "name", "template", "proxy", "profile", "options", "timeout", "noRedirects", "headers")
	if err != nil {
		return Fetch{}, err
	}

	useRaw, hasUse := m["use"]
	if !hasUse {
		return Fetch{}, vErr("fetch.use", "`fetch.use` is required")
	}
	use, _ := useRaw.(string)
	if err := enumField("fetch.use", use, "local", "proxy", "provider"); err != nil {
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

	timeout := getString(m, "timeout", "30s")
	if !validDuration(timeout) {
		return Fetch{}, vErr("fetch.timeout", "`fetch.timeout` is not a valid duration: %q", timeout)
	}
	fetch.Timeout = timeout

	fetch.NoRedirects = getBool(m, "noRedirects", false)

	if err := parseFetchHeaders(m, &fetch); err != nil {
		return Fetch{}, err
	}

	return fetch, nil
}

// profile only applies to the local/proxy tiers: rejected if set on
// provider, defaulted to "chrome" on local/proxy when left unset.
func parseFetchProfile(m map[string]any, use string, fetch *Fetch) *ValidationError {
	_, present := m["profile"]
	if err := requireTier(present, "fetch.profile", use, "local", "proxy"); err != nil {
		return err
	}
	switch {
	case present:
		profile := getString(m, "profile", "")
		if err := enumField("fetch.profile", profile, "chrome", "firefox", "safari"); err != nil {
			return err
		}
		fetch.Profile = profile
	case use == "local" || use == "proxy":
		fetch.Profile = "chrome"
	}
	return nil
}

// options only applies to the provider tier: rejected if set elsewhere,
// defaulted to an empty (not nil) map on provider when left unset — its
// contents are validated against the manifest, not here (pkg/jobspec has no
// manifest to check against).
func parseFetchOptions(m map[string]any, use string, fetch *Fetch) *ValidationError {
	_, present := m["options"]
	if err := requireTier(present, "fetch.options", use, "provider"); err != nil {
		return err
	}
	switch {
	case present:
		optMap, ok := m["options"].(map[string]any)
		if !ok {
			return vErr("fetch.options", "`fetch.options` must be a mapping")
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
		return vErr("fetch.headers", "`fetch.headers` must be a mapping")
	}
	headers := make(map[string]string, len(hdrMap))
	for k, val := range hdrMap {
		s, _ := val.(string)
		headers[k] = s
	}
	fetch.Headers = headers
	return nil
}
