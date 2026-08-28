package tiers

import (
	"context"
	"net/http"

	"github.com/pobear/pobear/pkg/fault"
	"github.com/pobear/pobear/pkg/jobspec"
	"github.com/pobear/pobear/pkg/provider"
)

type ManifestLookup interface {
	Lookup(name string) (*provider.Manifest, bool)
}

type CredentialLoader interface {
	Load(path, providerName, credential string) (secret string, ok bool, err error)
}

type Provider struct {
	Client          *http.Client
	Manifests       ManifestLookup
	Credentials     CredentialLoader
	CredentialsPath string
}

func (p *Provider) Fetch(ctx context.Context, cfg jobspec.Fetch, url string) (*Result, fault.Kind, error) {
	_ = ctx
	_ = cfg
	_ = url
	return nil, "", ErrNotImplemented
}
