package provider

import "errors"

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

var ErrNotImplemented = errors.New("provider: not implemented")

func Parse(data []byte) (*Manifest, error) {
	_ = data
	return nil, ErrNotImplemented
}
