package tiers

import (
	"context"
	"net/http"
	"net/url"

	"github.com/pobear/pobear/pkg/fault"
	"github.com/pobear/pobear/pkg/jobspec"
)

type Proxy struct {
	Client *http.Client
}

func (p *Proxy) Fetch(ctx context.Context, cfg jobspec.Fetch, target string) (*Result, fault.Kind, error) {
	proxyURL, err := url.Parse(cfg.Proxy)
	if err != nil {
		return nil, fault.Network, err
	}

	client := *p.Client
	transport := http.DefaultTransport
	if client.Transport != nil {
		transport = client.Transport
	}
	base, ok := transport.(*http.Transport)
	if !ok {
		base = http.DefaultTransport.(*http.Transport)
	}
	t := base.Clone()
	t.Proxy = http.ProxyURL(proxyURL)
	client.Transport = t

	return doHTTPFetch(ctx, &client, cfg, target)
}
