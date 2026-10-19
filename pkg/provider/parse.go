package provider

import (
	"bytes"
	"io"

	"gopkg.in/yaml.v3"

	"github.com/pobear/pobear/pkg/utils"
)

type ValidationError = utils.ValidationError

func Parse(data []byte) (*Manifest, error) {
	doc, verr := decodeSingleDocument(data)
	if verr != nil {
		return nil, verr
	}

	root, ok := doc.(map[string]any)
	if !ok {
		return nil, &ValidationError{Field: "$document", Message: "manifest document must be a mapping"}
	}

	if err := utils.CheckUnknownKeys(root, "", "name", "endpoint", "method", "auth", "headers", "options", "request", "response", "errors", "templates"); err != nil {
		return nil, err
	}

	nameRaw, hasName := root["name"]
	if !hasName {
		return nil, utils.Errorf("name", "`name` is required")
	}
	name, _ := nameRaw.(string)

	endpointRaw, hasEndpoint := root["endpoint"]
	if !hasEndpoint {
		return nil, utils.Errorf("endpoint", "`endpoint` is required")
	}
	endpoint, _ := endpointRaw.(string)
	if !utils.IsAbsoluteHTTPURL(endpoint) {
		return nil, utils.Errorf("endpoint", "`endpoint` must be an absolute `http`/`https` URL, got %q", endpoint)
	}

	method := utils.GetString(root, "method", "POST")

	auth, err := parseAuth(root)
	if err != nil {
		return nil, err
	}

	headers, err := parseHeaders(root)
	if err != nil {
		return nil, err
	}

	_, hasTemplates := root["templates"]
	_, hasTopOptions := root["options"]
	_, hasTopRequest := root["request"]
	if hasTemplates && (hasTopOptions || hasTopRequest) {
		return nil, utils.Errorf("templates", "`templates` must not be set together with top-level `options`/`request`")
	}

	response, err := parseResponse(root)
	if err != nil {
		return nil, err
	}

	errs, err := parseErrors(root)
	if err != nil {
		return nil, err
	}

	manifest := &Manifest{
		Name:     name,
		Endpoint: endpoint,
		Method:   method,
		Auth:     auth,
		Headers:  headers,
		Response: response,
		Errors:   errs,
	}

	if hasTemplates {
		templates, err := parseTemplates(root)
		if err != nil {
			return nil, err
		}
		manifest.Templates = templates
	} else {
		options, err := parseOptions(root, "options", "")
		if err != nil {
			return nil, err
		}
		manifest.Options = options

		request, err := parseRequest(root, "", true)
		if err != nil {
			return nil, err
		}
		manifest.Request = request
	}

	return manifest, nil
}

func decodeSingleDocument(data []byte) (any, *ValidationError) {
	dec := yaml.NewDecoder(bytes.NewReader(data))

	var first any
	if err := dec.Decode(&first); err != nil {
		if err == io.EOF {
			return nil, &ValidationError{Field: "$document", Message: "manifest document must contain exactly one YAML document"}
		}
		return nil, &ValidationError{Field: "$document", Message: "manifest document is not valid YAML: " + err.Error()}
	}

	var second any
	err := dec.Decode(&second)
	if err == nil {
		return nil, &ValidationError{Field: "$document", Message: "manifest document must contain exactly one YAML document"}
	}
	if err != io.EOF {
		return nil, &ValidationError{Field: "$document", Message: "manifest document is not valid YAML: " + err.Error()}
	}

	return first, nil
}

func parseHeaders(root map[string]any) (map[string]string, *ValidationError) {
	raw, ok := root["headers"]
	if !ok {
		return nil, nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, utils.Errorf("headers", "`headers` must be a mapping")
	}
	headers := make(map[string]string, len(m))
	for _, k := range utils.SortedKeys(m) {
		field := "headers." + k
		v, ok := m[k].(string)
		if !ok {
			return nil, utils.Errorf(field, "`%s` must be a string", field)
		}
		headers[k] = v
	}
	return headers, nil
}
