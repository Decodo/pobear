package job

import (
	"errors"

	"github.com/pobear/pobear/pkg/fault"
	"github.com/pobear/pobear/pkg/jobspec"
)

type SpecParser interface {
	Parse(data []byte, mode jobspec.Mode) (*jobspec.Spec, error)
}

var ErrNotImplemented = errors.New("job: not implemented")

type Parser struct {
	parse SpecParser
}

func NewParser(p SpecParser) *Parser {
	return &Parser{parse: p}
}

func (p *Parser) Parse(data []byte, mode jobspec.Mode) (*jobspec.Spec, fault.Kind, error) {
	_ = data
	_ = mode
	return nil, "", ErrNotImplemented
}
