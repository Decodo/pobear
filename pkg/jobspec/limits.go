package jobspec

import "github.com/Decodo/pobear/pkg/utils"

func parseLimits(root map[string]any) (Limits, *ValidationError) {
	raw, ok := root["limits"]
	if !ok {
		return Limits{Concurrency: 8, Retries: 3, Backoff: "exponential", PluginTimeout: "30s"}, nil
	}
	m, err := utils.AsMapping(raw, "limits", "concurrency", "rps", "retries", "backoff", "pluginConcurrency", "pluginTimeout")
	if err != nil {
		return Limits{}, err
	}

	concurrency, err := utils.GetInt(m, "concurrency", "limits.concurrency", 8)
	if err != nil {
		return Limits{}, err
	}
	if concurrency <= 0 {
		return Limits{}, utils.Errorf("limits.concurrency", "`limits.concurrency` must be > 0, got %d", concurrency)
	}

	rps, err := utils.GetInt(m, "rps", "limits.rps", 0)
	if err != nil {
		return Limits{}, err
	}
	if rps < 0 {
		return Limits{}, utils.Errorf("limits.rps", "`limits.rps` must be >= 0, got %d", rps)
	}

	retries, err := utils.GetInt(m, "retries", "limits.retries", 3)
	if err != nil {
		return Limits{}, err
	}
	if retries < 1 {
		return Limits{}, utils.Errorf("limits.retries", "`limits.retries` must be >= 1, got %d", retries)
	}

	backoff := utils.GetString(m, "backoff", "exponential")
	if backoff != "exponential" {
		return Limits{}, utils.Errorf("limits.backoff", "`limits.backoff` must be `exponential`, got %q", backoff)
	}

	pluginTimeout := utils.GetString(m, "pluginTimeout", "30s")
	if !utils.ValidDuration(pluginTimeout) {
		return Limits{}, utils.Errorf("limits.pluginTimeout", "`limits.pluginTimeout` is not a valid duration: %q", pluginTimeout)
	}

	pluginConcurrency, err := utils.GetInt(m, "pluginConcurrency", "limits.pluginConcurrency", 0)
	if err != nil {
		return Limits{}, err
	}

	return Limits{
		Concurrency:       concurrency,
		RPS:               rps,
		Retries:           retries,
		Backoff:           backoff,
		PluginConcurrency: pluginConcurrency,
		PluginTimeout:     pluginTimeout,
	}, nil
}
