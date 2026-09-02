package provider

import (
	"fmt"

	"github.com/pobear/pobear/pkg/fault"
)

func parseErrors(root map[string]any) ([]ErrorMapping, *ValidationError) {
	raw, ok := root["errors"]
	if !ok {
		return []ErrorMapping{}, nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, vErr("errors", "`errors` must be a list")
	}

	errs := make([]ErrorMapping, 0, len(list))
	for i, item := range list {
		field := fmt.Sprintf("errors[%d]", i)
		m, ok := item.(map[string]any)
		if !ok {
			return nil, vErr(field, "`%s` must be a mapping", field)
		}
		if err := checkUnknownKeys(m, field, "status", "kind"); err != nil {
			return nil, err
		}

		statusRaw, hasStatus := m["status"]
		if !hasStatus {
			return nil, vErr(field+".status", "`%s.status` is required", field)
		}
		status, _ := statusRaw.(int)
		if status < 100 || status > 599 {
			return nil, vErr(field+".status", "`%s.status` must be a real HTTP status code, got %d", field, status)
		}

		kindRaw, hasKind := m["kind"]
		if !hasKind {
			return nil, vErr(field+".kind", "`%s.kind` is required", field)
		}
		kind, _ := kindRaw.(string)
		if !fault.IsFetchOutcome(fault.Kind(kind)) {
			return nil, vErr(field+".kind", "`%s.kind` must be one of `auth`, `network`, `timeout`, `http`, `blocked`, `rateLimit`, `provider`, got %q", field, kind)
		}

		errs = append(errs, ErrorMapping{Status: status, Kind: kind})
	}
	return errs, nil
}
