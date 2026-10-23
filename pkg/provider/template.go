package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"strings"
	"sync"
	"text/template"

	"github.com/ohler55/ojg/jp"
)

func ApplyOptionDefaults(declared []Option, supplied map[string]any) map[string]any {
	out := make(map[string]any, len(supplied)+len(declared))
	maps.Copy(out, supplied)
	for i := range declared {
		opt := &declared[i]
		if opt.Default == nil {
			continue
		}
		if _, set := out[opt.Name]; set {
			continue
		}
		out[opt.Name] = opt.Default
	}
	return out
}

func Render(req Request, url string, options map[string]any) (body map[string]any, query map[string]string, err error) {
	data := struct {
		URL     string
		Options map[string]any
	}{URL: url, Options: options}

	body, err = renderValues(req.Body, data, options)
	if err != nil {
		return nil, nil, err
	}

	if req.Query != nil {
		query = make(map[string]string, len(req.Query))
		for k, v := range req.Query {
			s, ok := v.(string)
			if !ok {
				query[k] = fmt.Sprint(v)
				continue
			}
			rendered, err := renderTemplateString(s, data)
			if err != nil {
				return nil, nil, err
			}
			if rendered == "" {
				continue
			}
			query[k] = rendered
		}
	}

	return body, query, nil
}

var bareOptionsRefPattern = regexp.MustCompile(`^\{\{\s*\.Options\.([A-Za-z_][A-Za-z0-9_]*)\s*\}\}$`)

func renderValues(m map[string]any, data any, options map[string]any) (map[string]any, error) {
	if m == nil {
		return nil, nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		s, ok := v.(string)
		if !ok {
			out[k] = v
			continue
		}
		if match := bareOptionsRefPattern.FindStringSubmatch(s); match != nil {
			val, present := options[match[1]]
			if !present {
				continue
			}
			if str, isStr := val.(string); isStr && str == "" {
				continue
			}
			out[k] = val
			continue
		}
		rendered, err := renderTemplateString(s, data)
		if err != nil {
			return nil, err
		}
		if rendered == "" {
			continue
		}
		out[k] = rendered
	}
	return out, nil
}

var templateCache sync.Map

func compiledTemplate(s string) (*template.Template, error) {
	if cached, ok := templateCache.Load(s); ok {
		return cached.(*template.Template), nil
	}
	tmpl, err := template.New("").Parse(s)
	if err != nil {
		return nil, err
	}
	templateCache.Store(s, tmpl)
	return tmpl, nil
}

const noValuePlaceholder = "<no value>"

func renderTemplateString(s string, data any) (string, error) {
	tmpl, err := compiledTemplate(s)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	rendered := buf.String()
	if strings.Contains(rendered, noValuePlaceholder) {
		return "", nil
	}
	return rendered, nil
}

type ResolvedTemplate struct {
	Name     string
	Options  []Option
	Request  Request
	Endpoint string
	Method   string
}

type ErrTemplateNotFound struct {
	Manifest string
	Name     string
}

func (e *ErrTemplateNotFound) Error() string {
	return fmt.Sprintf("%s has no template %q", e.Manifest, e.Name)
}

type ErrTemplateAmbiguous struct {
	Manifest string
}

func (e *ErrTemplateAmbiguous) Error() string {
	return fmt.Sprintf("%s has more than one template and none is marked default", e.Manifest)
}

func ResolveTemplate(m *Manifest, templateName string) (*ResolvedTemplate, error) {
	if len(m.Templates) == 0 {
		return &ResolvedTemplate{Options: m.Options, Request: m.Request, Endpoint: m.Endpoint, Method: m.Method}, nil
	}

	var t *Template
	switch {
	case templateName != "":
		for i := range m.Templates {
			if m.Templates[i].Name == templateName {
				t = &m.Templates[i]
				break
			}
		}
		if t == nil {
			return nil, &ErrTemplateNotFound{Manifest: m.Name, Name: templateName}
		}
	case len(m.Templates) == 1:
		t = &m.Templates[0]
	default:
		for i := range m.Templates {
			if m.Templates[i].Default {
				t = &m.Templates[i]
				break
			}
		}
		if t == nil {
			return nil, &ErrTemplateAmbiguous{Manifest: m.Name}
		}
	}

	endpoint := t.Endpoint
	if endpoint == "" {
		endpoint = m.Endpoint
	}
	method := t.Method
	if method == "" {
		method = m.Method
	}
	return &ResolvedTemplate{Name: t.Name, Options: t.Options, Request: t.Request, Endpoint: endpoint, Method: method}, nil
}

func Extract(resp Response, httpStatus int, httpBody []byte, requestURL string) (status int, body []byte, finalURL string, bodyMatched bool, err error) {
	var parsed any
	if len(httpBody) > 0 {
		_ = json.Unmarshal(httpBody, &parsed)
	}

	status = httpStatus
	if resp.Status != "" {
		v, matched, err := extractJSONPath(resp.Status, parsed)
		if err != nil {
			return 0, nil, "", false, err
		}
		if matched {
			status = toInt(v)
		}
	}

	body = []byte{}
	if bodyVal, matched, err := extractJSONPath(resp.Body, parsed); err != nil {
		return 0, nil, "", false, err
	} else if matched {
		body, err = encodeBody(bodyVal)
		if err != nil {
			return 0, nil, "", false, err
		}
		bodyMatched = true
	}

	finalURL = requestURL
	if resp.FinalURL != "" {
		v, matched, err := extractJSONPath(resp.FinalURL, parsed)
		if err != nil {
			return 0, nil, "", false, err
		}
		if matched {
			finalURL = fmt.Sprint(v)
		}
	}

	return status, body, finalURL, bodyMatched, nil
}

func encodeBody(v any) ([]byte, error) {
	switch v.(type) {
	case map[string]any, []any:
		return json.Marshal(v)
	default:
		return fmt.Appendf(nil, "%v", v), nil
	}
}

func extractJSONPath(pathStr string, data any) (value any, matched bool, err error) {
	expr, err := jp.ParseString(pathStr)
	if err != nil {
		return nil, false, err
	}
	matches := expr.Get(data)
	if len(matches) == 0 {
		return nil, false, nil
	}
	return matches[0], true, nil
}

func toInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	default:
		return 0
	}
}
