package provider

import "errors"

// ErrTemplateNotImplemented is what Render and Extract return until a
// later phase implements them for real.
var ErrTemplateNotImplemented = errors.New("provider: template not implemented")

// Render renders req's body/query Go-templates (PROVIDER_REFERENCE.md
// §7-8) against url and options, producing the literal request body and
// query values to send; a template rendering to an empty string is
// dropped from query entirely, per §7. Stubbed until a later phase
// implements it for real.
func Render(req Request, url string, options map[string]any) (body map[string]any, query map[string]string, err error) {
	_ = req
	_ = url
	_ = options
	return nil, nil, ErrTemplateNotImplemented
}

// Extract resolves resp's JSONPath fields (PROVIDER_REFERENCE.md §9)
// against the raw HTTP response, producing the tier's logical
// status/body/finalURL — status falls back to httpStatus and finalURL
// falls back to requestURL when resp leaves them unset. Stubbed until a
// later phase implements it for real.
func Extract(resp Response, httpStatus int, httpBody []byte, requestURL string) (status int, body []byte, finalURL string, err error) {
	_ = resp
	_ = httpStatus
	_ = httpBody
	_ = requestURL
	return 0, nil, "", ErrTemplateNotImplemented
}
