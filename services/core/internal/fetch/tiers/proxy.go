package tiers

import (
	"context"
	"net/http"

	"github.com/Decodo/pobear/pkg/fault"
	"github.com/Decodo/pobear/pkg/jobspec"
)

type Proxy struct {
	Client *http.Client
}

func (p *Proxy) Fetch(ctx context.Context, cfg jobspec.Fetch, target string) (*Result, fault.Kind, error) {
	return doHTTPFetch(ctx, p.Client, cfg, target)
}
