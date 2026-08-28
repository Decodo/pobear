package extractors

import "errors"

var ErrNotImplemented = errors.New("extractors: not implemented")

type Result struct {
	Key   string
	Value any
}
