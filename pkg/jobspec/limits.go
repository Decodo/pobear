package jobspec

func parseLimits(root map[string]any) (Limits, *ValidationError) {
	raw, ok := root["limits"]
	if !ok {
		return Limits{Concurrency: 8, Retries: 3, Backoff: "exponential", PluginTimeout: "30s"}, nil
	}
	m, err := asMapping(raw, "limits", "concurrency", "rps", "retries", "backoff", "pluginConcurrency", "pluginTimeout")
	if err != nil {
		return Limits{}, err
	}

	concurrency := getInt(m, "concurrency", 8)
	if concurrency <= 0 {
		return Limits{}, vErr("limits.concurrency", "`limits.concurrency` must be > 0, got %d", concurrency)
	}

	rps := getInt(m, "rps", 0)
	if rps < 0 {
		return Limits{}, vErr("limits.rps", "`limits.rps` must be >= 0, got %d", rps)
	}

	retries := getInt(m, "retries", 3)
	if retries < 1 {
		return Limits{}, vErr("limits.retries", "`limits.retries` must be >= 1, got %d", retries)
	}

	backoff := getString(m, "backoff", "exponential")
	if backoff != "exponential" {
		return Limits{}, vErr("limits.backoff", "`limits.backoff` must be `exponential`, got %q", backoff)
	}

	pluginTimeout := getString(m, "pluginTimeout", "30s")
	if !validDuration(pluginTimeout) {
		return Limits{}, vErr("limits.pluginTimeout", "`limits.pluginTimeout` is not a valid duration: %q", pluginTimeout)
	}

	return Limits{
		Concurrency:       concurrency,
		RPS:               rps,
		Retries:           retries,
		Backoff:           backoff,
		PluginConcurrency: getInt(m, "pluginConcurrency", 0),
		PluginTimeout:     pluginTimeout,
	}, nil
}
