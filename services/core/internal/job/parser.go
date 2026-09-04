package job

import (
	"errors"

	"github.com/pobear/pobear/pkg/fault"
	"github.com/pobear/pobear/pkg/jobspec"
)

type SpecParser interface {
	Parse(data []byte, mode jobspec.Mode) (*jobspec.Spec, error)
}

type Parser struct {
	parse SpecParser
}

func NewParser(p SpecParser) *Parser {
	return &Parser{parse: p}
}

func (p *Parser) Parse(data []byte, mode jobspec.Mode) (*jobspec.Spec, fault.Kind, error) {
	spec, err := p.parse.Parse(data, mode)
	if err == nil {
		return spec, "", nil
	}

	if _, ok := errors.AsType[*jobspec.ValidationError](err); ok {
		return nil, fault.Spec, err
	}
	return nil, fault.Parse, err
}
