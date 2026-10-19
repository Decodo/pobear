package tiers

import (
	"context"
	"net/http"

	"github.com/pobear/pobear/pkg/fault"
	"github.com/pobear/pobear/pkg/jobspec"
)

type Local struct {
	Client *http.Client
}

func (l *Local) Fetch(ctx context.Context, cfg jobspec.Fetch, url string) (*Result, fault.Kind, error) {
	return doHTTPFetch(ctx, l.Client, cfg, url)
}
