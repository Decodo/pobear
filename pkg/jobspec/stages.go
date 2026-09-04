package jobspec

import (
	"fmt"

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
	return stages, nil
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
