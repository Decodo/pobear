package tiers

import (
	"context"
	"net/http"

	"github.com/pobear/pobear/pkg/fault"
	"github.com/pobear/pobear/pkg/jobspec"
)

type Proxy struct {
	Client *http.Client
}

func (p *Proxy) Fetch(ctx context.Context, cfg jobspec.Fetch, url string) (*Result, fault.Kind, error) {
	_ = ctx
	_ = cfg
	_ = url
	return nil, "", ErrNotImplemented
}
