package tiers

import "errors"

var ErrNotImplemented = errors.New("tiers: not implemented")

type Result struct {
	Status   int
	Body     []byte
	FinalURL string
}
