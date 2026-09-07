package jobspec

import (
	"bytes"
	"io"

	"gopkg.in/yaml.v3"

	"github.com/pobear/pobear/pkg/utils"
)

type Mode int

const (
	ModePlain Mode = iota
	ModeRPC
)

type ValidationError = utils.ValidationError

func Parse(data []byte, mode Mode) (*Spec, error) {
	doc, verr := decodeSingleDocument(data)
	if verr != nil {
		return nil, verr
	}

	root, ok := doc.(map[string]any)
	if !ok {
		return nil, &ValidationError{Field: "$document", Message: "spec document must be a mapping"}
	}

	if err := utils.CheckUnknownKeys(root, "", "version", "source", "fetch", "limits", "stages", "sink"); err != nil {
		return nil, err
	}

	version, err := parseVersion(root)
	if err != nil {
		return nil, err
	}
	source, err := parseSource(root)
	if err != nil {
		return nil, err
	}
	fetch, err := parseFetch(root)
	if err != nil {
		return nil, err
	}
	limits, err := parseLimits(root)
	if err != nil {
		return nil, err
	}
	stages, err := parseStages(root, mode)
	if err != nil {
		return nil, err
	}
	sink, err := parseSink(root, mode)
	if err != nil {
		return nil, err
	}

	return &Spec{
		Version: version,
		Source:  source,
		Fetch:   fetch,
		Limits:  limits,
		Stages:  stages,
		Sink:    sink,
	}, nil
}

func decodeSingleDocument(data []byte) (any, *ValidationError) {
	dec := yaml.NewDecoder(bytes.NewReader(data))

	var first any
	if err := dec.Decode(&first); err != nil {
		if err == io.EOF {
			return nil, &ValidationError{Field: "$document", Message: "spec document must contain exactly one YAML document"}
		}
		return nil, &ValidationError{Field: "$document", Message: "spec document is not valid YAML: " + err.Error()}
	}

	var second any
	err := dec.Decode(&second)
	if err == nil {
		return nil, &ValidationError{Field: "$document", Message: "spec document must contain exactly one YAML document"}
	}
	if err != io.EOF {
		return nil, &ValidationError{Field: "$document", Message: "spec document is not valid YAML: " + err.Error()}
	}

	return first, nil
}

func parseVersion(root map[string]any) (int, *ValidationError) {
	v, ok := root["version"]
	if !ok {
		return 0, utils.Errorf("version", "`version` is required")
	}
	n, ok := v.(int)
	if !ok || n != 1 {
		return 0, utils.Errorf("version", "`version` must be 1, got %v", v)
	}
	return n, nil
}
