package tiers

import (
	"context"
	"net/http"

	"github.com/Decodo/pobear/pkg/fault"
	"github.com/Decodo/pobear/pkg/jobspec"
)

type Local struct {
	Client *http.Client
}

func (l *Local) Fetch(ctx context.Context, cfg jobspec.Fetch, url string) (*Result, fault.Kind, error) {
	return doHTTPFetch(ctx, l.Client, cfg, url)
}
