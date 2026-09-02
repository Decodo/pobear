package provider

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"sort"

	"gopkg.in/yaml.v3"
)

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

func Parse(data []byte) (*Manifest, error) {
	doc, verr := decodeSingleDocument(data)
	if verr != nil {
		return nil, verr
	}

	root, ok := doc.(map[string]any)
	if !ok {
		return nil, &ValidationError{Field: "$document", Message: "manifest document must be a mapping"}
	}

	if err := checkUnknownKeys(root, "", "name", "endpoint", "method", "auth", "options", "request", "response", "errors", "templates"); err != nil {
		return nil, err
	}

	nameRaw, hasName := root["name"]
	if !hasName {
		return nil, vErr("name", "`name` is required")
	}
	name, _ := nameRaw.(string)

	endpointRaw, hasEndpoint := root["endpoint"]
	if !hasEndpoint {
		return nil, vErr("endpoint", "`endpoint` is required")
	}
	endpoint, _ := endpointRaw.(string)
	if !isAbsoluteHTTPURL(endpoint) {
		return nil, vErr("endpoint", "`endpoint` must be an absolute `http`/`https` URL, got %q", endpoint)
	}

	method := "POST"
	if v, ok := root["method"]; ok {
		method, _ = v.(string)
	}

	auth, err := parseAuth(root)
	if err != nil {
		return nil, err
	}

	_, hasTemplates := root["templates"]
	_, hasTopOptions := root["options"]
	_, hasTopRequest := root["request"]
	if hasTemplates && (hasTopOptions || hasTopRequest) {
		return nil, vErr("templates", "`templates` must not be set together with top-level `options`/`request`")
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

func vErr(field, format string, args ...any) *ValidationError {
	return &ValidationError{Field: field, Message: fmt.Sprintf(format, args...)}
}

func checkUnknownKeys(m map[string]any, prefix string, allowed ...string) *ValidationError {
	allowedSet := make(map[string]bool, len(allowed))
	for _, a := range allowed {
		allowedSet[a] = true
	}

	keys := sortedKeys(m)
	for _, k := range keys {
		if allowedSet[k] {
			continue
		}
		field := k
		if prefix != "" {
			field = prefix + "." + k
		}
		return vErr(field, "unknown field `%s`", field)
	}
	return nil
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func isAbsoluteHTTPURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	return u.IsAbs() && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
