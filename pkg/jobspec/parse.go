package jobspec

import "errors"

type Mode int

const (
	ModePlain Mode = iota
	ModeRPC
)

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

var ErrNotImplemented = errors.New("jobspec: Parse not implemented")

func Parse(data []byte, mode Mode) (*Spec, error) {
	_ = data
	_ = mode
	return nil, ErrNotImplemented
}
