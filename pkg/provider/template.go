package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"
	"text/template"

	"github.com/ohler55/ojg/jp"
)

func Render(req Request, url string, options map[string]any) (body map[string]any, query map[string]string, err error) {
	data := struct {
		URL     string
		Options map[string]any
	}{URL: url, Options: options}

	body, err = renderValues(req.Body, data)
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

func renderValues(m map[string]any, data any) (map[string]any, error) {
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
	if rendered == noValuePlaceholder {
		return "", nil
	}
	return rendered, nil
}

func Extract(resp Response, httpStatus int, httpBody []byte, requestURL string) (status int, body []byte, finalURL string, err error) {
	var parsed any
	if len(httpBody) > 0 {
		if err := json.Unmarshal(httpBody, &parsed); err != nil {
			return 0, nil, "", err
		}
	}

	status = httpStatus
	if resp.Status != "" {
		v, matched, err := extractJSONPath(resp.Status, parsed)
		if err != nil {
			return 0, nil, "", err
		}
		if matched {
			status = toInt(v)
		}
	}

	body = []byte{}
	if bodyVal, matched, err := extractJSONPath(resp.Body, parsed); err != nil {
		return 0, nil, "", err
	} else if matched {
		body = fmt.Appendf(nil, "%v", bodyVal)
	}

	finalURL = requestURL
	if resp.FinalURL != "" {
		v, matched, err := extractJSONPath(resp.FinalURL, parsed)
		if err != nil {
			return 0, nil, "", err
		}
		if matched {
			finalURL = fmt.Sprint(v)
		}
	}

	return status, body, finalURL, nil
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
