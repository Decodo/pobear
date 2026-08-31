package provider

import "errors"

var ErrTemplateNotImplemented = errors.New("provider: template not implemented")

func Render(req Request, url string, options map[string]any) (body map[string]any, query map[string]string, err error) {
	_ = req
	_ = url
	_ = options
	return nil, nil, ErrTemplateNotImplemented
}

func Extract(resp Response, httpStatus int, httpBody []byte, requestURL string) (status int, body []byte, finalURL string, err error) {
	_ = resp
	_ = httpStatus
	_ = httpBody
	_ = requestURL
	return 0, nil, "", ErrTemplateNotImplemented
}
