package jobspec

import (
	"fmt"
	"sort"

	"github.com/pobear/pobear/pkg/utils"
)

func parseStages(root map[string]any, mode Mode) ([]Stage, *ValidationError) {
	raw, ok := root["stages"]
	if !ok {
		return []Stage{}, nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, utils.Errorf("stages", "`stages` must be a list")
	}

	stages := make([]Stage, 0, len(list))
	for i, item := range list {
		field := fmt.Sprintf("stages[%d]", i)
		m, err := utils.AsMapping(item, field, "use", "fields", "id")
		if err != nil {
			return nil, err
		}

		use, _ := m["use"].(string)
		switch use {
		case "raw", "markdown":
		case "css":
			if _, ok := m["fields"]; !ok {
				return nil, utils.Errorf(field+".fields", "`%s.fields` is required for a `css` stage", field)
			}
		case "plugin":
			if mode != ModeRPC {
				return nil, utils.Errorf(field+".use", "`%s.use: plugin` is only allowed under `__rpc` mode", field)
			}
		default:
			return nil, utils.Errorf(field+".use", "`%s.use` must be one of `raw`, `markdown`, `css`, `plugin`, got %q", field, use)
		}

		stage := Stage{Use: use}
		if fieldsRaw, ok := m["fields"]; ok {
			fields, err := parseStringMap(fieldsRaw, field+".fields")
			if err != nil {
				return nil, err
			}
			stage.Fields = fields
		}
		if idRaw, ok := m["id"]; ok {
			stage.ID, _ = idRaw.(string)
		}

		stages = append(stages, stage)
	}

	if err := checkStageDataKeyCollisions(stages); err != nil {
		return nil, err
	}

	return stages, nil
}

func checkStageDataKeyCollisions(stages []Stage) *ValidationError {
	claimedBy := make(map[string]int)
	for i, s := range stages {
		for _, key := range stageDataKeys(s) {
			if prev, collided := claimedBy[key]; collided {
				field := fmt.Sprintf("stages[%d]", i)
				return utils.Errorf(field,
					"`%s` (%s) writes data key %q, already written by `stages[%d]` (%s) -- data keys must be unique across stages",
					field, s.Use, key, prev, stages[prev].Use)
			}
			claimedBy[key] = i
		}
	}
	return nil
}

func stageDataKeys(s Stage) []string {
	switch s.Use {
	case "raw":
		return []string{"raw"}
	case "markdown":
		return []string{"markdown"}
	case "css":
		keys := make([]string, 0, len(s.Fields))
		for k := range s.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return keys
	default:
		return nil
	}
}

func parseStringMap(raw any, field string) (map[string]string, *ValidationError) {
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, utils.Errorf(field, "`%s` must be a mapping", field)
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		s, ok := v.(string)
		if !ok {
			return nil, utils.Errorf(field, "`%s.%s` must be a string", field, k)
		}
		out[k] = s
	}
	return out, nil
}
