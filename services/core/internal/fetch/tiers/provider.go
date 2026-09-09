package tiers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

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

func (p *Provider) Fetch(ctx context.Context, cfg jobspec.Fetch, targetURL string) (*Result, fault.Kind, error) {
	manifest, err := p.lookupManifest(cfg.Name)
	if err != nil {
		return nil, fault.Provider, err
	}

	secret, err := p.loadCredential(manifest)
	if err != nil {
		return nil, fault.Auth, err
	}

	tmplReq, endpoint, method, err := resolveProviderTemplate(manifest, cfg.Template)
	if err != nil {
		return nil, fault.Provider, err
	}

	body, query, err := provider.Render(tmplReq, targetURL, cfg.Options)
	if err != nil {
		return nil, fault.Provider, err
	}

	req, kind, err := buildHTTPRequest(ctx, manifest, endpoint, method, cfg, body, query, secret)
	if err != nil {
		return nil, kind, err
	}

	resp, err := p.Client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fault.Timeout, err
		}
		return nil, fault.Network, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fault.Network, err
	}

	status, extractedBody, finalURL, err := provider.Extract(manifest.Response, resp.StatusCode, respBody, targetURL)
	if err != nil {
		return nil, fault.Provider, err
	}

	result := &Result{Status: status, Body: extractedBody, FinalURL: finalURL}

	respKind := classifyProviderStatus(status, manifest.Errors)
	if respKind == "" {
		return result, "", nil
	}
	if snippet := bodySnippet(respBody); snippet != "" {
		return result, respKind, fmt.Errorf("tiers: provider status %d: %s", status, snippet)
	}
	return result, respKind, fmt.Errorf("tiers: provider status %d", status)
}

func (p *Provider) lookupManifest(name string) (*provider.Manifest, error) {
	if p.Manifests != nil {
		if m, ok := p.Manifests.Lookup(name); ok {
			return m, nil
		}
	}
	return nil, fmt.Errorf("tiers: unknown provider %q", name)
}

func (p *Provider) loadCredential(manifest *provider.Manifest) (string, error) {
	secret, ok, err := p.Credentials.Load(p.CredentialsPath, manifest.Name, manifest.Auth.Credential)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("tiers: missing credential for provider %q", manifest.Name)
	}
	return secret, nil
}

func buildHTTPRequest(ctx context.Context, manifest *provider.Manifest, endpoint, method string, cfg jobspec.Fetch, body map[string]any, query map[string]string, secret string) (*http.Request, fault.Kind, error) {
	endpoint, err := applyQuery(endpoint, query)
	if err != nil {
		return nil, fault.Provider, err
	}

	var bodyReader io.Reader
	if len(body) > 0 {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fault.Provider, err
		}
		bodyReader = bytes.NewReader(encoded)
	}

	if method == "" {
		method = http.MethodPost
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, bodyReader)
	if err != nil {
		return nil, fault.Network, err
	}
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	applyHeaders(req, cfg.Headers)
	if err := applyAuth(req, manifest.Auth, secret); err != nil {
		return nil, fault.Auth, err
	}
	return req, "", nil
}

func applyQuery(endpoint string, query map[string]string) (string, error) {
	if len(query) == 0 {
		return endpoint, nil
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	q := u.Query()
	for k, v := range query {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func applyAuth(req *http.Request, auth provider.Auth, secret string) error {
	switch auth.Type {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+secret)
	case "basic":
		req.Header.Set("Authorization", "Basic "+secret)
	case "header":
		req.Header.Set(auth.Name, secret)
	case "query":
		q := req.URL.Query()
		q.Set(auth.Name, secret)
		req.URL.RawQuery = q.Encode()
	default:
		return fmt.Errorf("tiers: unknown auth type %q", auth.Type)
	}
	return nil
}

func resolveProviderTemplate(m *provider.Manifest, templateName string) (req provider.Request, endpoint, method string, err error) {
	rt, err := provider.ResolveTemplate(m, templateName)
	if err != nil {
		return provider.Request{}, "", "", wrapTemplateSelectionError(err)
	}
	return rt.Request, rt.Endpoint, rt.Method, nil
}

func wrapTemplateSelectionError(err error) error {
	switch e := err.(type) {
	case *provider.ErrTemplateNotFound:
		return fmt.Errorf("tiers: %s has no template %q", e.Manifest, e.Name)
	case *provider.ErrTemplateAmbiguous:
		return fmt.Errorf("tiers: %s has more than one template and none is marked default; fetch.template is required", e.Manifest)
	default:
		return err
	}
}

func classifyProviderStatus(status int, mappings []provider.ErrorMapping) fault.Kind {
	for _, m := range mappings {
		if m.Status == status {
			return fault.Kind(m.Kind)
		}
	}
	return classifyDefaultStatus(status)
}
